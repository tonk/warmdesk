package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

const errResolutionCodeRequired = "a resolution code is required to resolve this ticket"

// ticketUpdateRequest is the body of PUT /customers/:id/tickets/:id and
// PUT /tickets/inbox/:id. Every field is optional; for the optional foreign
// keys (assignee, owner, group, category, resolution code) 0 clears.
type ticketUpdateRequest struct {
	Title           *string         `json:"title"`
	Description     *string         `json:"description"`
	Type            *string         `json:"type"`
	Status          *string         `json:"status"`
	Priority        *string         `json:"priority"`
	Impact          *string         `json:"impact"`
	Urgency         *string         `json:"urgency"`
	CategoryID      *uint           `json:"category_id"`
	ResolutionCode  *uint           `json:"resolution_code_id"`
	ResolutionNotes *string         `json:"resolution_notes"`
	IsMajorIncident *bool           `json:"is_major_incident"`
	ProblemStatus   *string         `json:"problem_status"`
	RootCause       *string         `json:"root_cause"`
	Workaround      *string         `json:"workaround"`
	IsKnownError    *bool           `json:"is_known_error"`
	AssignedTo      *uint           `json:"assigned_to_id"`
	OwnerID         *uint           `json:"owner_id"`
	GroupID         *uint           `json:"group_id"`
	ReminderAt      json.RawMessage `json:"reminder_at"`
	CloseAt         json.RawMessage `json:"close_at"`
	// CustomerID assigns an inbox ticket to a customer (inbox endpoint only).
	CustomerID *uint `json:"customer_id"`
}

func ptrEq(a *uint, b uint) bool { return (a == nil && b == 0) || (a != nil && *a == b) }

func idOrNil(id uint) *uint {
	if id == 0 {
		return nil
	}
	return &id
}

func userDisplayName(id uint) string {
	if id == 0 {
		return "unassigned"
	}
	var u models.User
	database.DB.First(&u, id)
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// ticketUpdateError is a rejected ticket update: the HTTP status and JSON body to send.
type ticketUpdateError struct {
	Status int
	Body   gin.H
}

func badUpdate(msg string) *ticketUpdateError {
	return &ticketUpdateError{http.StatusBadRequest, gin.H{"error": msg}}
}

// applyTicketUpdate validates req against ticket, persists the changes and
// records history. It returns a non-nil error (nothing persisted) when the
// request is rejected. allowCustomerAssign enables customer_id (inbox
// tickets only).
func applyTicketUpdate(ticket *models.Ticket, req *ticketUpdateRequest, userID uint, allowCustomerAssign bool) *ticketUpdateError {
	now := time.Now()
	settings := services.LoadHelpdeskSettings()
	updates := map[string]any{}
	var history []models.TicketHistory
	logEvent := func(event, detail string) {
		history = append(history, models.TicketHistory{TicketID: ticket.ID, UserID: userID, EventType: event, Detail: detail})
	}
	slaInputsChanged := false
	majorIncidentRaised := false

	if req.Title != nil && *req.Title != ticket.Title {
		updates["title"] = *req.Title
		ticket.Title = *req.Title
		logEvent("title_changed", *req.Title)
	}
	if req.Description != nil && *req.Description != ticket.Description {
		updates["description"] = *req.Description
		ticket.Description = *req.Description
	}
	if req.Type != nil && services.ValidTicketTypes[*req.Type] && *req.Type != ticket.Type {
		updates["type"] = *req.Type
		ticket.Type = *req.Type
		slaInputsChanged = true
		logEvent("type_changed", *req.Type)
		if ticket.Type == "problem" && ticket.ProblemStatus == "" {
			ticket.ProblemStatus = "investigating"
			updates["problem_status"] = ticket.ProblemStatus
		}
	}
	if req.CustomerID != nil && allowCustomerAssign && *req.CustomerID > 0 {
		var cust models.Customer
		if database.DB.First(&cust, *req.CustomerID).Error != nil {
			return badUpdate("customer not found")
		}
		updates["customer_id"] = *req.CustomerID
		ticket.CustomerID = req.CustomerID
		slaInputsChanged = true
		logEvent("customer_assigned", strconv.FormatUint(uint64(*req.CustomerID), 10))
	}
	if req.CategoryID != nil && !ptrEq(ticket.CategoryID, *req.CategoryID) {
		name := ""
		if *req.CategoryID != 0 {
			var cat models.TicketCategory
			if database.DB.First(&cat, *req.CategoryID).Error != nil {
				return badUpdate("category not found")
			}
			name = cat.Name
		}
		ticket.CategoryID = idOrNil(*req.CategoryID)
		updates["category_id"] = ticket.CategoryID
		slaInputsChanged = true
		logEvent("category_changed", name)
	}

	// Priority: an explicit priority wins; otherwise a changed impact/urgency
	// pair derives it from the priority matrix.
	impactChanged := false
	for _, f := range []struct {
		val   *string
		field *string
		col   string
	}{{req.Impact, &ticket.Impact, "impact"}, {req.Urgency, &ticket.Urgency, "urgency"}} {
		if f.val == nil || *f.val == *f.field {
			continue
		}
		if *f.val != "" && !services.ValidImpactUrgency[*f.val] {
			return badUpdate("invalid " + f.col)
		}
		*f.field = *f.val
		updates[f.col] = *f.val
		impactChanged = true
		logEvent(f.col+"_changed", *f.val)
	}
	newPriority := ""
	if req.Priority != nil && services.ValidTicketPriorities[*req.Priority] {
		newPriority = *req.Priority
	} else if impactChanged {
		if p, ok := services.DerivePriority(settings.PriorityMatrix, ticket.Impact, ticket.Urgency); ok {
			newPriority = p
		}
	}
	if newPriority != "" && newPriority != ticket.Priority {
		updates["priority"] = newPriority
		ticket.Priority = newPriority
		slaInputsChanged = true
		logEvent("priority_changed", newPriority)
	}

	if req.ResolutionCode != nil && !ptrEq(ticket.ResolutionCodeID, *req.ResolutionCode) {
		name := ""
		if *req.ResolutionCode != 0 {
			var rc models.TicketResolutionCode
			if database.DB.First(&rc, *req.ResolutionCode).Error != nil {
				return badUpdate("resolution code not found")
			}
			name = rc.Name
		}
		ticket.ResolutionCodeID = idOrNil(*req.ResolutionCode)
		updates["resolution_code_id"] = ticket.ResolutionCodeID
		logEvent("resolution_changed", name)
	}
	if req.ResolutionNotes != nil && *req.ResolutionNotes != ticket.ResolutionNotes {
		ticket.ResolutionNotes = *req.ResolutionNotes
		updates["resolution_notes"] = *req.ResolutionNotes
	}
	if req.IsMajorIncident != nil && *req.IsMajorIncident != ticket.IsMajorIncident {
		ticket.IsMajorIncident = *req.IsMajorIncident
		updates["is_major_incident"] = *req.IsMajorIncident
		majorIncidentRaised = *req.IsMajorIncident
		logEvent("major_incident_changed", strconv.FormatBool(*req.IsMajorIncident))
	}

	// Problem-management fields.
	if req.ProblemStatus != nil && *req.ProblemStatus != ticket.ProblemStatus {
		if *req.ProblemStatus != "" && !services.ValidProblemStatuses[*req.ProblemStatus] {
			return badUpdate("invalid problem_status")
		}
		ticket.ProblemStatus = *req.ProblemStatus
		updates["problem_status"] = *req.ProblemStatus
		logEvent("problem_status_changed", *req.ProblemStatus)
		if *req.ProblemStatus == "known_error" && req.IsKnownError == nil && !ticket.IsKnownError {
			ticket.IsKnownError = true
			updates["is_known_error"] = true
		}
	}
	if req.RootCause != nil && *req.RootCause != ticket.RootCause {
		ticket.RootCause = *req.RootCause
		updates["root_cause"] = *req.RootCause
		logEvent("root_cause_changed", "")
	}
	if req.Workaround != nil && *req.Workaround != ticket.Workaround {
		ticket.Workaround = *req.Workaround
		updates["workaround"] = *req.Workaround
		logEvent("workaround_changed", "")
	}
	if req.IsKnownError != nil && *req.IsKnownError != ticket.IsKnownError {
		ticket.IsKnownError = *req.IsKnownError
		updates["is_known_error"] = *req.IsKnownError
		logEvent("known_error_changed", strconv.FormatBool(*req.IsKnownError))
	}

	// Assignment.
	if req.AssignedTo != nil && !ptrEq(ticket.AssignedToID, *req.AssignedTo) {
		ticket.AssignedToID = idOrNil(*req.AssignedTo)
		updates["assigned_to_id"] = ticket.AssignedToID
		logEvent("assignee_changed", userDisplayName(*req.AssignedTo))
	}
	if req.OwnerID != nil && !ptrEq(ticket.OwnerID, *req.OwnerID) {
		ticket.OwnerID = idOrNil(*req.OwnerID)
		updates["owner_id"] = ticket.OwnerID
		logEvent("owner_changed", userDisplayName(*req.OwnerID))
	}
	if req.GroupID != nil && !ptrEq(ticket.GroupID, *req.GroupID) {
		detail := "unassigned"
		if *req.GroupID != 0 {
			var g models.UserGroup
			database.DB.First(&g, *req.GroupID)
			detail = g.Name
		}
		ticket.GroupID = idOrNil(*req.GroupID)
		updates["group_id"] = ticket.GroupID
		logEvent("group_changed", detail)
	}

	datesChanged := false
	if t, ok := parseNullableTimestamp(req.ReminderAt); ok {
		updates["reminder_at"] = t // nil clears
		ticket.ReminderAt = t
		datesChanged = true
	}
	if t, ok := parseNullableTimestamp(req.CloseAt); ok {
		updates["close_at"] = t // nil clears
		ticket.CloseAt = t
		datesChanged = true
	}
	if datesChanged {
		logEvent("dates_changed", "reminder / close")
	}

	// SLA policy/deadlines follow the ticket's matching inputs, always counted
	// from created_at. Applied before the status change so a pause banked by
	// the transition shifts the new resolution deadline.
	if slaInputsChanged {
		for k, v := range services.ApplySla(ticket) {
			updates[k] = v
		}
	}

	if req.Status != nil && services.ValidTicketStatuses[*req.Status] && *req.Status != ticket.Status {
		newStatus := *req.Status
		if services.StatusRequiresChecklistComplete(newStatus) && services.TicketChecklistBlocksClose(ticket.ID) {
			return badUpdate(errChecklistIncomplete)
		}
		if settings.RequireResolutionCode && services.IsResolvedStatus(newStatus) &&
			!services.IsResolvedStatus(ticket.Status) && ticket.ResolutionCodeID == nil {
			return &ticketUpdateError{http.StatusBadRequest, gin.H{"error": errResolutionCodeRequired, "code": "resolution_code_required"}}
		}
		for k, v := range services.TransitionTicketStatus(ticket, newStatus, now, services.TransitionOptions{CountReopen: true}) {
			updates[k] = v
		}
		logEvent("status_changed", newStatus)
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error; err != nil {
			return &ticketUpdateError{http.StatusInternalServerError, gin.H{"error": "failed to update ticket"}}
		}
	}
	for i := range history {
		database.DB.Create(&history[i])
	}
	services.RefreshSlaBreaches(ticket, now)
	if majorIncidentRaised {
		t := *ticket
		go services.NotifyMajorIncident(&t)
	}
	return nil
}

// preloadTicket loads a ticket with every association the detail/list
// responses carry.
func preloadTicket(id uint) (models.Ticket, error) {
	var t models.Ticket
	err := database.DB.Preload("CreatedBy").Preload("AssignedTo").Preload("Owner").Preload("Group").
		Preload("Tags").Preload("SlaPolicy").Preload("Category").Preload("ResolutionCode").
		First(&t, id).Error
	return t, err
}

// isAgentReply reports whether a message posted by userID counts as the
// helpdesk's first response: a public message from someone other than a
// customer-portal user or the ticket's own creator. Email tickets are
// "created" by the system user (the first admin) on the requester's behalf,
// so there any agent's reply counts — including that admin's.
func isAgentReply(ticket *models.Ticket, userID uint, role string, isPrivate bool) bool {
	if isPrivate || role == "customer" {
		return false
	}
	return userID != ticket.CreatedByID || (ticket.FromEmail != nil && *ticket.FromEmail != "")
}

// onTicketMessage applies the lifecycle side effects of a new message: the
// first agent reply stamps first_response_at; a requester reply reopens a
// resolved ticket (counted as a reopen) or resumes a pending one.
func onTicketMessage(ticket *models.Ticket, userID uint, role string, isPrivate bool) {
	now := time.Now()
	if ticket.FirstResponseAt == nil && isAgentReply(ticket, userID, role, isPrivate) {
		ticket.FirstResponseAt = &now
		database.DB.Model(&models.Ticket{}).Where("id = ?", ticket.ID).Update("first_response_at", now)
	}
	if role == "customer" && (ticket.Status == "pending" || services.IsResolvedStatus(ticket.Status)) {
		services.SaveTicketStatus(ticket, "open", userID, services.TransitionOptions{CountReopen: true})
	}
	services.RefreshSlaBreaches(ticket, now)
}

// trimmedOrEmpty is a small helper for optional free-text query parameters.
func trimmedOrEmpty(s string) string { return strings.TrimSpace(s) }

// initNewTicket fills the classification fields of a ticket about to be
// created (type, impact/urgency, priority, category) and its SLA. Unknown
// type/priority values fall back to defaults the way ticket creation always
// has; an invalid impact/urgency or unknown category is reported as errMsg.
func initNewTicket(t *models.Ticket, ticketType, defaultType, priority, impact, urgency string, categoryID *uint) (errMsg string) {
	if !services.ValidTicketTypes[ticketType] {
		ticketType = defaultType
	}
	t.Type = ticketType
	if t.Type == "problem" {
		t.ProblemStatus = "investigating"
	}
	for _, v := range []string{impact, urgency} {
		if v != "" && !services.ValidImpactUrgency[v] {
			return "invalid impact/urgency"
		}
	}
	t.Impact, t.Urgency = impact, urgency
	switch {
	case services.ValidTicketPriorities[priority]:
		t.Priority = priority
	default:
		t.Priority = "medium"
		if p, ok := services.DerivePriority(services.LoadHelpdeskSettings().PriorityMatrix, impact, urgency); ok {
			t.Priority = p
		}
	}
	if categoryID != nil && *categoryID != 0 {
		if database.DB.First(&models.TicketCategory{}, *categoryID).Error != nil {
			return "category not found"
		}
		t.CategoryID = categoryID
	}
	services.ApplySla(t)
	return ""
}
