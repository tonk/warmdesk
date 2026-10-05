package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

// group creates a user group with the given members.
func (f *itilFixture) group(t *testing.T, name string, members ...models.User) models.UserGroup {
	t.Helper()
	g := models.UserGroup{Name: name}
	require.NoError(t, database.DB.Create(&g).Error)
	for _, m := range members {
		require.NoError(t, database.DB.Create(&models.GroupMember{GroupID: g.ID, UserID: m.ID}).Error)
	}
	return g
}

func (f *itilFixture) user(t *testing.T, name string) models.User {
	t.Helper()
	u := models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", GlobalRole: "user"}
	require.NoError(t, database.DB.Create(&u).Error)
	return u
}

func reloadTicket(t *testing.T, id uint) models.Ticket {
	t.Helper()
	var tk models.Ticket
	require.NoError(t, database.DB.First(&tk, id).Error)
	return tk
}

func TestServiceRequestWithApprovalRound(t *testing.T) {
	f := newITILFixture(t)
	mgr1, mgr2 := f.user(t, "mgr1"), f.user(t, "mgr2")
	approvers := f.group(t, "Managers", mgr1, mgr2)
	fulfil := f.group(t, "Service desk", f.agent)
	tmpl := models.TicketChecklistTemplate{Name: "Laptop", Items: models.TicketChecklistTemplateItems{"Order", "Image", "Hand over"}, IsActive: true}
	require.NoError(t, database.DB.Create(&tmpl).Error)
	item := models.ServiceCatalogItem{
		Name: "New laptop", IsActive: true, DefaultPriority: "low", GroupID: &fulfil.ID,
		ApprovalGroupID: &approvers.ID, ApprovalsRequired: 2, ChecklistTemplateID: &tmpl.ID,
		Fields: models.CatalogFields{
			{Key: "model", Label: "Model", Type: "select", Required: true, Options: []string{"13 inch", "15 inch"}},
			{Key: "needed_by", Label: "Needed by", Type: "date"},
			{Key: "dock", Label: "Docking station", Type: "checkbox"},
		},
	}
	require.NoError(t, database.DB.Create(&item).Error)
	params := map[string]string{"customerId": idStr(f.acme.ID)}

	code, body := call(t, CreateServiceRequest, f.portal, http.MethodPost, `{"catalog_item_id":`+idStr(item.ID)+`,"answers":{"model":"17 inch"}}`, params, "")
	require.Equal(t, http.StatusBadRequest, code, string(body))
	code, _ = call(t, CreateServiceRequest, f.portal, http.MethodPost, `{"catalog_item_id":`+idStr(item.ID)+`,"answers":{}}`, params, "")
	require.Equal(t, http.StatusBadRequest, code, "model is required")

	// A customer-portal user may order from the catalogue.
	code, body = call(t, CreateServiceRequest, f.portal, http.MethodPost,
		`{"catalog_item_id":`+idStr(item.ID)+`,"description":"For the new hire","answers":{"model":"15 inch","needed_by":"2026-11-01","dock":true}}`, params, "")
	require.Equal(t, http.StatusCreated, code, string(body))
	var req models.Ticket
	require.NoError(t, json.Unmarshal(body, &req))
	assert.Equal(t, "service_request", req.Type)
	assert.Equal(t, "low", req.Priority)
	assert.Equal(t, "New laptop", req.Title)
	assert.Contains(t, req.Description, "**Model:** 15 inch")
	assert.Contains(t, req.Description, "**Docking station:** yes")
	assert.Equal(t, "15 inch", req.RequestData["model"])
	require.NotNil(t, req.GroupID)
	assert.Equal(t, fulfil.ID, *req.GroupID)
	assert.Equal(t, services.ApprovalPending, req.ApprovalStatus)
	assert.Len(t, req.ChecklistItems, 3)

	// Fulfilment can't start before approval.
	code, body = call(t, UpdateTicket, f.agent, http.MethodPut, `{"status":"in_progress"}`, ticketParams(req), "")
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "approval_required")

	// Non-approvers can't decide; one approval of two keeps it pending.
	approve := func(u models.User, payload string) (int, []byte) {
		return call(t, DecideApproval, u, http.MethodPost, payload, map[string]string{"ticketId": idStr(req.ID)}, "")
	}
	code, _ = approve(f.agent, `{"decision":"approved"}`)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = approve(mgr1, `{"decision":"approved"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, services.ApprovalPending, reloadTicket(t, req.ID).ApprovalStatus)

	// The approver sees it in their list.
	var mine []approvalEntry
	_, body = call(t, ListMyApprovals, mgr2, http.MethodGet, "", nil, "")
	require.NoError(t, json.Unmarshal(body, &mine))
	require.Len(t, mine, 1)
	assert.Equal(t, "New laptop", mine[0].CatalogItemName)
	assert.Equal(t, 1, mine[0].Approved)

	code, _ = approve(mgr2, `{"decision":"approved","comment":"ok"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, services.ApprovalApproved, reloadTicket(t, req.ID).ApprovalStatus)
	code, _ = approve(mgr2, `{"decision":"rejected","comment":"late"}`)
	assert.Equal(t, http.StatusConflict, code, "a settled round takes no more decisions")

	code, body = call(t, UpdateTicket, f.agent, http.MethodPut, `{"status":"in_progress"}`, ticketParams(req), "")
	require.Equal(t, http.StatusOK, code, string(body))
}

func TestApprovalRejectionNeedsReasonAndRejects(t *testing.T) {
	f := newITILFixture(t)
	mgr := f.user(t, "mgr")
	g := f.group(t, "Approvers", mgr)
	tk := f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Type = "service_request" })

	code, _ := call(t, RequestTicketApproval, f.agent, http.MethodPost, `{"group_id":`+idStr(g.ID)+`,"approvals_required":3}`, ticketParams(tk), "")
	require.Equal(t, http.StatusOK, code)
	params := map[string]string{"ticketId": idStr(tk.ID)}
	code, body := call(t, DecideApproval, mgr, http.MethodPost, `{"decision":"rejected"}`, params, "")
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "comment_required")
	code, _ = call(t, DecideApproval, mgr, http.MethodPost, `{"decision":"rejected","comment":"no budget"}`, params, "")
	require.Equal(t, http.StatusOK, code)
	tk = reloadTicket(t, tk.ID)
	assert.Equal(t, services.ApprovalRejected, tk.ApprovalStatus)

	// A rejected request can still be closed, not resolved.
	code, _ = call(t, UpdateTicket, f.agent, http.MethodPut, `{"status":"resolved"}`, ticketParams(tk), "")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, UpdateTicket, f.agent, http.MethodPut, `{"status":"closed"}`, ticketParams(tk), "")
	assert.Equal(t, http.StatusOK, code)
}

func TestChangeLifecycle(t *testing.T) {
	f := newITILFixture(t)
	cab1 := f.user(t, "cab1")
	cab := f.group(t, "CAB", cab1)
	put := func(tk models.Ticket, payload string) (int, models.Ticket, string) {
		code, body := call(t, UpdateTicket, f.agent, http.MethodPut, payload, ticketParams(tk), "")
		var out models.Ticket
		_ = json.Unmarshal(body, &out)
		return code, out, string(body)
	}

	code, body := call(t, CreateTicket, f.agent, http.MethodPost, `{"title":"Upgrade DB","type":"change_request"}`, map[string]string{"customerId": idStr(f.acme.ID)}, "")
	require.Equal(t, http.StatusCreated, code, string(body))
	var ch models.Ticket
	require.NoError(t, json.Unmarshal(body, &ch))
	assert.Equal(t, "draft", ch.ChangeStatus)
	assert.Equal(t, "normal", ch.ChangeType)

	code, _, msg := put(ch, `{"change_status":"awaiting_approval"}`)
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "change_incomplete")

	code, _, msg = put(ch, `{"implementation_plan":"run migration","backout_plan":"restore backup","risk":"medium","change_status":"awaiting_approval"}`)
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "no_cab_group")

	// The rejected update is atomic: nothing of it was saved.
	assert.Empty(t, reloadTicket(t, ch.ID).ImplementationPlan)

	saveSetting(services.SettingChangeCABGroup, idStr(cab.ID))
	code, ch, msg = put(ch, `{"implementation_plan":"run migration","backout_plan":"restore backup","risk":"medium","change_status":"awaiting_approval"}`)
	require.Equal(t, http.StatusOK, code, msg)
	assert.Equal(t, "awaiting_approval", ch.ChangeStatus)
	assert.Equal(t, services.ApprovalPending, ch.ApprovalStatus)

	code, _ = call(t, DecideApproval, cab1, http.MethodPost, `{"decision":"approved"}`, map[string]string{"ticketId": idStr(ch.ID)}, "")
	require.Equal(t, http.StatusOK, code)
	ch = reloadTicket(t, ch.ID)
	assert.Equal(t, "approved", ch.ChangeStatus)

	code, _, msg = put(ch, `{"change_status":"scheduled"}`)
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "planned_window_required")

	// A freeze window blocks scheduling a normal change.
	start := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Hour)
	require.NoError(t, database.DB.Create(&models.ChangeFreezeWindow{Name: "Year end", StartAt: start.Add(-time.Hour), EndAt: start.Add(time.Hour)}).Error)
	window := `"planned_start":"` + start.Format(time.RFC3339) + `","planned_end":"` + start.Add(2*time.Hour).Format(time.RFC3339) + `"`
	code, _, msg = put(ch, `{`+window+`,"change_status":"scheduled"}`)
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "change_freeze")

	later := start.Add(72 * time.Hour)
	window = `"planned_start":"` + later.Format(time.RFC3339) + `","planned_end":"` + later.Add(2*time.Hour).Format(time.RFC3339) + `"`
	code, ch, msg = put(ch, `{`+window+`,"change_status":"scheduled"}`)
	require.Equal(t, http.StatusOK, code, msg)
	assert.Equal(t, "scheduled", ch.ChangeStatus)

	code, ch, msg = put(ch, `{"change_status":"implementing"}`)
	require.Equal(t, http.StatusOK, code, msg)
	assert.Equal(t, "in_progress", ch.Status)
	code, ch, _ = put(ch, `{"change_status":"review"}`)
	require.Equal(t, http.StatusOK, code)
	code, _, msg = put(ch, `{"change_status":"completed"}`)
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "outcome_required")
	code, ch, msg = put(ch, `{"change_outcome":"successful","review_notes":"went fine","change_status":"completed"}`)
	require.Equal(t, http.StatusOK, code, msg)
	assert.Equal(t, "completed", ch.ChangeStatus)
	assert.Equal(t, "resolved", ch.Status)
	assert.NotNil(t, ch.ResolvedAt)

	// Skipping steps is refused.
	code, _, msg = put(ch, `{"change_status":"implementing"}`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, msg, "invalid_change_transition")
}

func TestStandardChangeIsPreApproved(t *testing.T) {
	f := newITILFixture(t)
	ch := f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.Type = "change_request"
		tk.ChangeStatus = "draft"
		tk.ChangeType = "normal"
	})
	code, body := call(t, UpdateTicket, f.agent, http.MethodPut, `{"change_type":"standard","change_status":"awaiting_approval"}`, ticketParams(ch), "")
	require.Equal(t, http.StatusOK, code, string(body))
	ch = reloadTicket(t, ch.ID)
	assert.Equal(t, "approved", ch.ChangeStatus)
	assert.Equal(t, services.ApprovalApproved, ch.ApprovalStatus)

	code, body = call(t, UpdateTicket, f.agent, http.MethodPut, `{"change_type":"normal"}`, ticketParams(ch), "")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "change_not_draft")
}

func TestKnowledgeBaseVisibility(t *testing.T) {
	f := newITILFixture(t)
	create := func(payload string) models.KnowledgeArticle {
		code, body := call(t, CreateKnowledgeArticle, f.agent, http.MethodPost, payload, nil, "")
		require.Equal(t, http.StatusCreated, code, string(body))
		var a models.KnowledgeArticle
		require.NoError(t, json.Unmarshal(body, &a))
		return a
	}
	public := create(`{"title":"Reset your VPN password","body":"Go to the portal","status":"published","visibility":"public","keywords":"vpn password"}`)
	assert.NotNil(t, public.PublishedAt)
	internal := create(`{"title":"VPN concentrator runbook","body":"ssh to vpn01","status":"published","visibility":"internal"}`)
	draft := create(`{"title":"VPN draft","visibility":"public"}`)
	acmeOnly := create(`{"title":"Acme VPN profile","status":"published","visibility":"public","customer_id":` + idStr(f.acme.ID) + `}`)

	// Globex-only article; the agent has no Globex access, so the admin writes it.
	code, body := call(t, CreateKnowledgeArticle, f.admin, http.MethodPost, `{"title":"Globex VPN","status":"published","visibility":"public","customer_id":`+idStr(f.globex.ID)+`}`, nil, "")
	require.Equal(t, http.StatusCreated, code, string(body))
	var globexOnly models.KnowledgeArticle
	require.NoError(t, json.Unmarshal(body, &globexOnly))

	ids := func(user models.User, query string) []uint {
		_, body := call(t, ListKnowledgeArticles, user, http.MethodGet, "", nil, query)
		var arts []models.KnowledgeArticle
		require.NoError(t, json.Unmarshal(body, &arts))
		out := []uint{}
		for _, a := range arts {
			out = append(out, a.ID)
			assert.Empty(t, a.Body, "list responses omit the body")
		}
		return out
	}
	assert.ElementsMatch(t, []uint{public.ID, acmeOnly.ID}, ids(f.portal, ""))
	assert.ElementsMatch(t, []uint{public.ID, internal.ID, draft.ID, acmeOnly.ID, globexOnly.ID}, ids(f.agent, ""))
	assert.Equal(t, []uint{public.ID}, ids(f.portal, "q=password"))

	code, _ = call(t, GetKnowledgeArticle, f.portal, http.MethodGet, "", map[string]string{"id": idStr(internal.ID)}, "")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, GetKnowledgeArticle, f.portal, http.MethodGet, "", map[string]string{"id": idStr(globexOnly.ID)}, "")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, CreateKnowledgeArticle, f.portal, http.MethodPost, `{"title":"x"}`, nil, "")
	assert.Equal(t, http.StatusForbidden, code)

	// Suggestions rank title matches first and skip drafts.
	_, body = call(t, SuggestKnowledgeArticles, f.agent, http.MethodGet, "", nil, "q=vpn+password+reset")
	var sugg []models.KnowledgeArticle
	require.NoError(t, json.Unmarshal(body, &sugg))
	require.NotEmpty(t, sugg)
	assert.Equal(t, public.ID, sugg[0].ID)
	for _, s := range sugg {
		assert.NotEqual(t, draft.ID, s.ID)
	}

	code, body = call(t, KnowledgeArticleFeedback, f.portal, http.MethodPost, `{"helpful":true}`, map[string]string{"id": idStr(public.ID)}, "")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(body), `"helpful_yes":1`)

	// Only the author or an admin deletes.
	other := f.user(t, "other")
	code, _ = call(t, DeleteKnowledgeArticle, other, http.MethodDelete, "", map[string]string{"id": idStr(public.ID)}, "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = call(t, DeleteKnowledgeArticle, f.agent, http.MethodDelete, "", map[string]string{"id": idStr(public.ID)}, "")
	assert.Equal(t, http.StatusOK, code)
}

func TestCMDBAccessImpactAndChangeConflicts(t *testing.T) {
	f := newITILFixture(t)
	SeedCITypes()
	var server, app models.CIType
	require.NoError(t, database.DB.Where("name = ?", "Server").First(&server).Error)
	require.NoError(t, database.DB.Where("name = ?", "Application").First(&app).Error)

	mk := func(user models.User, payload string) models.ConfigurationItem {
		code, body := call(t, CreateCI, user, http.MethodPost, payload, nil, "")
		require.Equal(t, http.StatusCreated, code, string(body))
		var ci models.ConfigurationItem
		require.NoError(t, json.Unmarshal(body, &ci))
		return ci
	}
	db := mk(f.agent, `{"name":"db01","type_id":`+idStr(server.ID)+`,"customer_id":`+idStr(f.acme.ID)+`,"attributes":{"os":"Debian"}}`)
	crm := mk(f.agent, `{"name":"CRM","type_id":`+idStr(app.ID)+`,"customer_id":`+idStr(f.acme.ID)+`}`)
	portal := mk(f.agent, `{"name":"Portal","type_id":`+idStr(app.ID)+`}`)
	foreign := mk(f.admin, `{"name":"globex-web","type_id":`+idStr(server.ID)+`,"customer_id":`+idStr(f.globex.ID)+`}`)

	code, _ := call(t, CreateCI, f.agent, http.MethodPost, `{"name":"x","type_id":`+idStr(server.ID)+`,"customer_id":`+idStr(f.globex.ID)+`}`, nil, "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = call(t, GetCI, f.agent, http.MethodGet, "", map[string]string{"id": idStr(foreign.ID)}, "")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, ListCIs, f.portal, http.MethodGet, "", nil, "")
	assert.Equal(t, http.StatusForbidden, code, "the CMDB is agents only")

	rel := func(src, dst models.ConfigurationItem, typ string) {
		code, body := call(t, CreateCIRelation, f.admin, http.MethodPost, `{"target_id":`+idStr(dst.ID)+`,"type":"`+typ+`"}`, map[string]string{"id": idStr(src.ID)}, "")
		require.Equal(t, http.StatusCreated, code, string(body))
	}
	rel(crm, db, "depends_on")     // CRM depends on db01
	rel(portal, crm, "depends_on") // Portal depends on CRM
	rel(foreign, db, "runs_on")    // hidden from the agent

	code, body := call(t, GetCIImpact, f.agent, http.MethodGet, "", map[string]string{"id": idStr(db.ID)}, "")
	require.Equal(t, http.StatusOK, code)
	var impact struct {
		Impacted []ciImpactEntry `json:"impacted"`
		Hidden   int             `json:"hidden"`
	}
	require.NoError(t, json.Unmarshal(body, &impact))
	require.Len(t, impact.Impacted, 2)
	assert.Equal(t, "CRM", impact.Impacted[0].CI.Name)
	assert.Equal(t, 1, impact.Impacted[0].Depth)
	assert.Equal(t, "Portal", impact.Impacted[1].CI.Name)
	assert.Equal(t, 2, impact.Impacted[1].Depth)
	assert.Equal(t, 1, impact.Hidden)

	// Two scheduled changes on the same CI in overlapping windows conflict.
	start := time.Now().Add(24 * time.Hour)
	end := start.Add(2 * time.Hour)
	a := f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.Type, tk.ChangeStatus, tk.PlannedStart, tk.PlannedEnd = "change_request", "scheduled", &start, &end
	})
	b := f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.Type, tk.ChangeStatus, tk.PlannedStart, tk.PlannedEnd = "change_request", "draft", &start, &end
	})
	for _, tk := range []models.Ticket{a, b} {
		code, body := call(t, LinkTicketCI, f.agent, http.MethodPost, `{"ci_id":`+idStr(db.ID)+`}`, ticketParams(tk), "")
		require.Equal(t, http.StatusCreated, code, string(body))
	}
	code, body = call(t, CheckChangeSchedule, f.agent, http.MethodGet, "", ticketParams(b), "")
	require.Equal(t, http.StatusOK, code)
	var check services.ScheduleCheck
	require.NoError(t, json.Unmarshal(body, &check))
	require.Len(t, check.Conflicts, 1)
	assert.Equal(t, a.ID, check.Conflicts[0].TicketID)
	assert.Equal(t, []string{"db01"}, check.Conflicts[0].CIs)

	code, _ = call(t, LinkTicketCI, f.agent, http.MethodPost, `{"ci_id":`+idStr(foreign.ID)+`}`, ticketParams(a), "")
	assert.Equal(t, http.StatusNotFound, code)

	// The change calendar shows both changes.
	_, body = call(t, ListChanges, f.agent, http.MethodGet, "", nil, "")
	var cal struct {
		Changes []changeCalendarEntry `json:"changes"`
	}
	require.NoError(t, json.Unmarshal(body, &cal))
	assert.Len(t, cal.Changes, 2)
}
