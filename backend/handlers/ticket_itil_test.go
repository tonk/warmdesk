package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"github.com/tonk/warmdesk/testutil"
)

type itilFixture struct {
	admin, agent, portal models.User
	acme, globex         models.Customer
}

// newITILFixture sets up an admin, an agent with access to Acme only, a
// customer-portal user for Acme, and two customers.
func newITILFixture(t *testing.T) *itilFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	prev := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = prev
		cleanup()
	})
	f := &itilFixture{
		admin:  models.User{Username: "admin", Email: "admin@example.com", PasswordHash: "x", GlobalRole: "admin"},
		agent:  models.User{Username: "agent", Email: "agent@example.com", PasswordHash: "x", GlobalRole: "user"},
		portal: models.User{Username: "portal", Email: "portal@example.com", PasswordHash: "x", GlobalRole: "customer"},
		acme:   models.Customer{Name: "Acme"},
		globex: models.Customer{Name: "Globex"},
	}
	for _, u := range []*models.User{&f.admin, &f.agent, &f.portal} {
		require.NoError(t, db.Create(u).Error)
	}
	require.NoError(t, db.Create(&f.acme).Error)
	require.NoError(t, db.Create(&f.globex).Error)
	require.NoError(t, db.Create(&models.CustomerAccess{UserID: f.agent.ID, CustomerID: f.acme.ID, Role: "member"}).Error)
	require.NoError(t, db.Create(&models.CustomerAccess{UserID: f.portal.ID, CustomerID: f.acme.ID, Role: "member"}).Error)
	return f
}

func (f *itilFixture) ticket(t *testing.T, cust *models.Customer, mut func(*models.Ticket)) models.Ticket {
	t.Helper()
	tk := models.Ticket{CustomerID: &cust.ID, Title: "T", Status: "open", Priority: "medium", Type: "incident", CreatedByID: f.admin.ID}
	if mut != nil {
		mut(&tk)
	}
	require.NoError(t, database.DB.Create(&tk).Error)
	return tk
}

func call(t *testing.T, h gin.HandlerFunc, user models.User, method, body string, params map[string]string, query string) (int, []byte) {
	t.Helper()
	c, w := ginTestContext(t, method, "/x?"+query, body)
	c.Set(middleware.ContextUserID, user.ID)
	c.Set(middleware.ContextGlobalRole, user.GlobalRole)
	for k, v := range params {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}
	h(c)
	return w.Code, w.Body.Bytes()
}

// ticketParamsID returns the ticket_id of a message response body.
func ticketParamsID(t *testing.T, body []byte) uint {
	t.Helper()
	var m struct {
		TicketID uint `json:"ticket_id"`
	}
	require.NoError(t, json.Unmarshal(body, &m))
	return m.TicketID
}

func idStr(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func ticketParams(tk models.Ticket) map[string]string {
	return map[string]string{"customerId": idStr(*tk.CustomerID), "ticketId": idStr(tk.ID)}
}

func TestUpdateTicketDerivesPriorityAndKeepsDeadlineFromCreation(t *testing.T) {
	f := newITILFixture(t)
	require.NoError(t, database.DB.Create(&models.SlaPolicy{Name: "Medium", PriorityFilter: "medium", ResolutionTimeMinutes: 24 * 60, IsActive: true}).Error)
	require.NoError(t, database.DB.Create(&models.SlaPolicy{Name: "Critical", PriorityFilter: "critical", ResolutionTimeMinutes: 4 * 60, IsActive: true}).Error)
	created := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	tk := f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.CreatedAt = created
		services.ApplySla(tk)
	})

	code, body := call(t, UpdateTicket, f.agent, http.MethodPut, `{"impact":"high","urgency":"high"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusOK, code, string(body))
	var got models.Ticket
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "critical", got.Priority)
	require.NotNil(t, got.SlaPolicy)
	assert.Equal(t, "Critical", got.SlaPolicy.Name)
	require.NotNil(t, got.SlaResolutionDeadline)
	assert.WithinDuration(t, created.Add(4*time.Hour), *got.SlaResolutionDeadline, time.Second)

	// An explicit priority overrides the matrix.
	code, body = call(t, UpdateTicket, f.agent, http.MethodPut, `{"impact":"low","priority":"high"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusOK, code, string(body))
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "high", got.Priority)
	assert.Equal(t, "low", got.Impact)

	code, _ = call(t, UpdateTicket, f.agent, http.MethodPut, `{"impact":"enormous"}`, ticketParams(tk), "")
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestResolveRequiresResolutionCodeWhenConfigured(t *testing.T) {
	f := newITILFixture(t)
	saveSetting(services.SettingHelpdeskRequireResolution, "true")
	rc := models.TicketResolutionCode{Name: "Fixed", IsActive: true}
	require.NoError(t, database.DB.Create(&rc).Error)
	tk := f.ticket(t, &f.acme, nil)

	code, body := call(t, UpdateTicket, f.agent, http.MethodPut, `{"status":"resolved"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "resolution_code_required")

	code, body = call(t, UpdateTicket, f.agent, http.MethodPut,
		`{"status":"resolved","resolution_code_id":`+idStr(rc.ID)+`,"resolution_notes":"rebooted"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusOK, code, string(body))
	var got models.Ticket
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "resolved", got.Status)
	assert.NotNil(t, got.ResolvedAt)
	require.NotNil(t, got.ResolutionCode)
	assert.Equal(t, "Fixed", got.ResolutionCode.Name)
}

func TestCustomerReplyReopensResolvedTicket(t *testing.T) {
	f := newITILFixture(t)
	now := time.Now()
	tk := f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Status = "resolved"; tk.ResolvedAt = &now })

	code, body := call(t, CreateTicketMessage, f.portal, http.MethodPost, `{"body":"still broken"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusCreated, code, string(body))
	tk = models.Ticket{}
	database.DB.First(&tk, ticketParamsID(t, body))
	assert.Equal(t, "open", tk.Status)
	assert.Equal(t, 1, tk.ReopenCount)
	assert.Nil(t, tk.ResolvedAt)
	assert.Nil(t, tk.FirstResponseAt, "a requester reply is not a first response")

	code, _ = call(t, CreateTicketMessage, f.agent, http.MethodPost, `{"body":"on it"}`, ticketParams(tk), "")
	require.Equal(t, http.StatusCreated, code)
	id := tk.ID
	tk = models.Ticket{}
	database.DB.First(&tk, id)
	assert.NotNil(t, tk.FirstResponseAt, "first response is recorded even without an SLA")
}

func TestProblemsAreHiddenFromCustomerRole(t *testing.T) {
	f := newITILFixture(t)
	f.ticket(t, &f.acme, nil)
	problem := f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Type = "problem"; tk.Title = "Root" })

	code, body := call(t, ListTickets, f.portal, http.MethodGet, "", map[string]string{"customerId": idStr(f.acme.ID)}, "")
	require.Equal(t, http.StatusOK, code)
	var list []models.Ticket
	require.NoError(t, json.Unmarshal(body, &list))
	assert.Len(t, list, 1)
	assert.Equal(t, "incident", list[0].Type)

	code, _ = call(t, GetTicket, f.portal, http.MethodGet, "", ticketParams(problem), "")
	assert.Equal(t, http.StatusNotFound, code)

	code, _ = call(t, GetTicket, f.agent, http.MethodGet, "", ticketParams(problem), "")
	assert.Equal(t, http.StatusOK, code)
}

func TestTicketLinksRespectAccessAndDirection(t *testing.T) {
	f := newITILFixture(t)
	incident := f.ticket(t, &f.acme, nil)
	problem := f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Type = "problem"; tk.Workaround = "restart" })
	foreign := f.ticket(t, &f.globex, nil)

	// The agent has no access to Globex: its tickets look non-existent.
	code, _ := call(t, CreateTicketLink, f.agent, http.MethodPost, `{"target_ticket_id":`+idStr(foreign.ID)+`}`, ticketParams(incident), "")
	assert.Equal(t, http.StatusNotFound, code)

	code, _ = call(t, CreateTicketLink, f.agent, http.MethodPost, `{"target_ticket_id":`+idStr(problem.ID)+`,"link_type":"blocks"}`, ticketParams(incident), "")
	assert.Equal(t, http.StatusBadRequest, code)

	code, body := call(t, CreateTicketLink, f.agent, http.MethodPost, `{"target_ticket_id":`+idStr(problem.ID)+`,"link_type":"caused_by"}`, ticketParams(incident), "")
	require.Equal(t, http.StatusCreated, code, string(body))

	// An admin may link across customers.
	code, _ = call(t, CreateTicketLink, f.admin, http.MethodPost, `{"target_ticket_id":`+idStr(problem.ID)+`,"link_type":"caused_by"}`, ticketParams(foreign), "")
	require.Equal(t, http.StatusCreated, code)

	var links []ticketLinkEntry
	_, body = call(t, ListTicketLinks, f.agent, http.MethodGet, "", ticketParams(incident), "")
	require.NoError(t, json.Unmarshal(body, &links))
	require.Len(t, links, 1)
	assert.Equal(t, "caused_by", links[0].LinkType)
	assert.Equal(t, "outgoing", links[0].Direction)
	assert.Equal(t, "restart", links[0].Workaround)

	// Seen from the problem, the agent only sees the incident it can access.
	_, body = call(t, ListTicketLinks, f.agent, http.MethodGet, "", ticketParams(problem), "")
	require.NoError(t, json.Unmarshal(body, &links))
	require.Len(t, links, 1)
	assert.Equal(t, "incoming", links[0].Direction)
	assert.Equal(t, incident.ID, links[0].ID)

	// The portal user never sees the problem link.
	_, body = call(t, ListTicketLinks, f.portal, http.MethodGet, "", ticketParams(incident), "")
	require.NoError(t, json.Unmarshal(body, &links))
	assert.Empty(t, links)
}

func TestResolveLinkedIncidents(t *testing.T) {
	f := newITILFixture(t)
	problem := f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Type = "problem" })
	a := f.ticket(t, &f.acme, nil)
	b := f.ticket(t, &f.acme, nil)
	unrelated := f.ticket(t, &f.acme, nil)
	foreign := f.ticket(t, &f.globex, nil)
	for _, inc := range []models.Ticket{a, b, foreign} {
		require.NoError(t, database.DB.Create(&models.TicketLink{SourceTicketID: inc.ID, TargetTicketID: problem.ID, LinkType: "caused_by"}).Error)
	}
	require.NoError(t, database.DB.Create(&models.TicketLink{SourceTicketID: unrelated.ID, TargetTicketID: problem.ID, LinkType: "related"}).Error)

	code, body := call(t, ResolveLinkedIncidents, f.agent, http.MethodPost, `{"message":"Fixed upstream"}`, ticketParams(problem), "")
	require.Equal(t, http.StatusOK, code, string(body))
	var out struct{ Resolved, Skipped int }
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Equal(t, 2, out.Resolved)
	assert.Equal(t, 1, out.Skipped, "the Globex incident is out of the agent's reach")

	for _, tc := range []struct {
		tk   models.Ticket
		want string
	}{{a, "resolved"}, {b, "resolved"}, {unrelated, "open"}, {foreign, "open"}} {
		database.DB.First(&tc.tk, tc.tk.ID)
		assert.Equal(t, tc.want, tc.tk.Status, "ticket %d", tc.tk.ID)
	}
	var msgs int64
	database.DB.Model(&models.TicketMessage{}).Where("ticket_id = ? AND body = ?", a.ID, "Fixed upstream").Count(&msgs)
	assert.Equal(t, int64(1), msgs)

	// Only problems can resolve linked incidents.
	code, _ = call(t, ResolveLinkedIncidents, f.agent, http.MethodPost, `{}`, ticketParams(a), "")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestListProblemsScopesToAccessibleCustomers(t *testing.T) {
	f := newITILFixture(t)
	f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.Type = "problem"
		tk.Title = "VPN drops"
		tk.IsKnownError = true
		tk.Workaround = "reconnect"
	})
	f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.Type = "problem"; tk.Title = "Slow login" })
	f.ticket(t, &f.globex, func(tk *models.Ticket) { tk.Type = "problem"; tk.Title = "Globex outage"; tk.IsKnownError = true })

	var list []problemEntry
	_, body := call(t, ListProblems, f.agent, http.MethodGet, "", nil, "")
	require.NoError(t, json.Unmarshal(body, &list))
	assert.Len(t, list, 2)

	_, body = call(t, ListProblems, f.agent, http.MethodGet, "", nil, "known_error=true&q=reconnect")
	require.NoError(t, json.Unmarshal(body, &list))
	require.Len(t, list, 1)
	assert.Equal(t, "VPN drops", list[0].Title)

	_, body = call(t, ListProblems, f.admin, http.MethodGet, "", nil, "known_error=true")
	require.NoError(t, json.Unmarshal(body, &list))
	assert.Len(t, list, 2)

	code, _ := call(t, ListProblems, f.portal, http.MethodGet, "", nil, "")
	assert.Equal(t, http.StatusForbidden, code)
}

func TestTicketReportKPIs(t *testing.T) {
	f := newITILFixture(t)
	now := time.Now()
	day := 24 * time.Hour
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	// Resolved in 2h, one agent reply, SLA met → counts towards FCR.
	fast := f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.CreatedAt = now.Add(-3 * day)
		tk.Status, tk.ResolvedAt, tk.FirstResponseAt = "closed", ago(3*day-2*time.Hour), ago(3*day-time.Hour)
		tk.SlaResolutionDeadline = ago(2 * day)
		tk.Priority = "high"
	})
	database.DB.Create(&models.TicketMessage{TicketID: fast.ID, UserID: f.agent.ID, Body: "fixed"})
	// Resolved in 4h, reopened once, resolution SLA missed.
	f.ticket(t, &f.acme, func(tk *models.Ticket) {
		tk.CreatedAt = now.Add(-2 * day)
		tk.Status, tk.ResolvedAt, tk.ReopenCount = "resolved", ago(2*day-4*time.Hour), 1
		tk.SlaResolutionDeadline = ago(2*day - time.Hour)
	})
	// Still open, created 10 days ago, major incident.
	f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.CreatedAt = now.Add(-10 * day); tk.IsMajorIncident = true })
	// Globex ticket the agent can't see.
	f.ticket(t, &f.globex, func(tk *models.Ticket) { tk.CreatedAt = now.Add(-day) })
	// Spam is ignored.
	f.ticket(t, &f.acme, func(tk *models.Ticket) { tk.CreatedAt = now.Add(-day); tk.IsSpam = true })

	code, body := call(t, GetTicketReport, f.agent, http.MethodGet, "", nil, "")
	require.Equal(t, http.StatusOK, code, string(body))
	var r ticketReport
	require.NoError(t, json.Unmarshal(body, &r))
	assert.Equal(t, 3, r.Created)
	assert.Equal(t, 2, r.Resolved)
	assert.Equal(t, 1, r.Backlog)
	assert.Equal(t, 1, r.MajorIncidents)
	require.NotNil(t, r.MTTRMinutes)
	assert.Equal(t, 180, *r.MTTRMinutes)
	require.NotNil(t, r.ResolutionCompliance)
	assert.InDelta(t, 50, *r.ResolutionCompliance, 0.01)
	require.NotNil(t, r.FCRRate)
	assert.InDelta(t, 50, *r.FCRRate, 0.01)
	require.NotNil(t, r.ReopenRate)
	assert.InDelta(t, 50, *r.ReopenRate, 0.01)
	assert.Equal(t, 1, r.Ageing[3].Count)
	require.NotEmpty(t, r.ByPriority)
	assert.Equal(t, "high", r.ByPriority[0].Key)

	// Asking for an inaccessible customer is refused; the exports render.
	code, _ = call(t, GetTicketReport, f.agent, http.MethodGet, "", nil, "customer_id="+idStr(f.globex.ID))
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = call(t, GetTicketReportPDF, f.agent, http.MethodGet, "", nil, "lang=nl")
	assert.Equal(t, http.StatusOK, code)
	code, _ = call(t, GetTicketReportXLSX, f.agent, http.MethodGet, "", nil, "lang=de")
	assert.Equal(t, http.StatusOK, code)
	code, _ = call(t, GetTicketReport, f.portal, http.MethodGet, "", nil, "")
	assert.Equal(t, http.StatusForbidden, code)
}

func TestMacroStatusChangeGoesThroughLifecycle(t *testing.T) {
	f := newITILFixture(t)
	tk := f.ticket(t, &f.acme, nil)
	macro := models.Macro{Name: "Resolve", IsActive: true, Actions: models.MacroActions{{Type: "set_status", Value: "resolved"}, {Type: "set_impact", Value: "high"}, {Type: "set_urgency", Value: "low"}}}
	require.NoError(t, database.DB.Create(&macro).Error)

	params := ticketParams(tk)
	params["macroId"] = idStr(macro.ID)
	code, body := call(t, ApplyMacro, f.agent, http.MethodPost, "", params, "")
	require.Equal(t, http.StatusOK, code, string(body))
	database.DB.First(&tk, tk.ID)
	assert.Equal(t, "resolved", tk.Status)
	assert.NotNil(t, tk.ResolvedAt)
	assert.Equal(t, "medium", tk.Priority, "high impact × low urgency")
	var hist int64
	database.DB.Model(&models.TicketHistory{}).Where("ticket_id = ? AND event_type = ?", tk.ID, "status_changed").Count(&hist)
	assert.Equal(t, int64(1), hist)
}

func TestBusinessCalendarValidation(t *testing.T) {
	f := newITILFixture(t)
	code, body := call(t, AdminCreateBusinessCalendar, f.admin, http.MethodPost, `{"name":"Office","timezone":"Europe/Amsterdam","work_days":"1,2,3,4,5","start_time":"09:00","end_time":"17:00","holidays":[{"date":"2026-12-25","name":"Christmas"}]}`, nil, "")
	require.Equal(t, http.StatusCreated, code, string(body))
	var cal models.BusinessCalendar
	require.NoError(t, json.Unmarshal(body, &cal))
	assert.Len(t, cal.Holidays, 1)

	code, _ = call(t, AdminCreateBusinessCalendar, f.admin, http.MethodPost, `{"name":"Broken","work_days":"","start_time":"09:00","end_time":"17:00"}`, nil, "")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, AdminCreateBusinessCalendar, f.admin, http.MethodPost, `{"name":"Mars","timezone":"Mars/Olympus"}`, nil, "")
	assert.Equal(t, http.StatusBadRequest, code)

	code, body = call(t, AdminImportCalendarHolidays, f.admin, http.MethodPost, `{"year":2027,"locale":"nl"}`, map[string]string{"id": idStr(cal.ID)}, "")
	require.Equal(t, http.StatusOK, code, string(body))
	var out struct {
		Added int `json:"added"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	assert.Greater(t, out.Added, 5)
}
