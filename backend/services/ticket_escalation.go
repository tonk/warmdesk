package services

import (
	"fmt"
	"html"
	"log"
	"time"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	appws "github.com/tonk/warmdesk/ws"
)

// Ticket alert kinds, sent as the "kind" of a ticket.alert WebSocket message.
const (
	TicketAlertSlaWarning    = "sla_warning"
	TicketAlertSlaBreach     = "sla_breach"
	TicketAlertMajorIncident = "major_incident"
)

// StartTicketSlaScheduler runs the SLA job once a minute: auto-closing,
// breach evaluation, threshold warnings and escalation. Breach flags used to
// be refreshed only when a ticket was viewed; this keeps them current (and
// escalation timely) even for tickets nobody is looking at.
func StartTicketSlaScheduler() {
	go func() {
		BackfillTicketTimestamps()
		runTicketSlaJob()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			runTicketSlaJob()
		}
	}()
}

func runTicketSlaJob() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("helpdesk: SLA job panicked: %v", r)
		}
	}()
	AutoCloseTickets()
	ProcessSlaTickets(time.Now())
}

// ProcessSlaTickets evaluates every unresolved ticket under an SLA policy:
// refreshes breach flags, sends the threshold warning once, and escalates
// once on the first breach.
func ProcessSlaTickets(now time.Time) {
	var tickets []models.Ticket
	database.DB.Where("sla_policy_id IS NOT NULL AND status NOT IN ? AND (is_spam = ? OR is_spam IS NULL)",
		[]string{"resolved", "pending_close", "closed"}, false).Find(&tickets)
	policies := map[uint]*models.SlaPolicy{}
	calendars := map[uint]*WorkCalendar{}
	for i := range tickets {
		t := &tickets[i]
		policy, ok := policies[*t.SlaPolicyID]
		if !ok {
			var p models.SlaPolicy
			if database.DB.First(&p, *t.SlaPolicyID).Error == nil {
				policy = &p
			}
			policies[*t.SlaPolicyID] = policy
			calendars[*t.SlaPolicyID] = policyCalendar(policy)
		}
		RefreshSlaBreaches(t, now)
		if policy == nil {
			continue
		}
		cal := calendars[*t.SlaPolicyID]
		breached := t.SlaResponseBreached || t.SlaResolutionBreached
		switch {
		case breached && t.SlaEscalatedAt == nil:
			escalateTicket(t, policy, now)
		case !breached && t.SlaWarnedAt == nil && slaWarningDue(t, policy, cal, now):
			warnTicket(t, now)
		}
	}
}

// slaWarningDue reports whether the share of SLA time used on either clock
// has reached the policy's warning threshold.
func slaWarningDue(t *models.Ticket, policy *models.SlaPolicy, cal *WorkCalendar, now time.Time) bool {
	if policy.WarningPercent <= 0 {
		return false
	}
	threshold := float64(policy.WarningPercent) / 100
	created := t.CreatedAt
	if t.FirstResponseAt == nil && policy.ResponseTimeMinutes > 0 {
		used := float64(BusinessMinutesBetween(cal, created, now)) / float64(policy.ResponseTimeMinutes)
		if used >= threshold {
			return true
		}
	}
	if t.SlaPausedAt == nil && policy.ResolutionTimeMinutes > 0 {
		used := float64(BusinessMinutesBetween(cal, created, now)-t.SlaPausedMinutes) / float64(policy.ResolutionTimeMinutes)
		if used >= threshold {
			return true
		}
	}
	return false
}

// claimTicketAlert sets column to now only if it is still NULL, so when
// several instances run the scheduler exactly one of them sends the alert.
func claimTicketAlert(ticketID uint, column string, now time.Time) bool {
	res := database.DB.Model(&models.Ticket{}).Where("id = ? AND "+column+" IS NULL", ticketID).Update(column, now)
	return res.Error == nil && res.RowsAffected == 1
}

func warnTicket(t *models.Ticket, now time.Time) {
	if !claimTicketAlert(t.ID, "sla_warned_at", now) {
		return
	}
	t.SlaWarnedAt = &now
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: SystemUserID(), EventType: "sla_warning"})
	NotifyTicketAlert(t, ticketResponsibles(t), TicketAlertSlaWarning)
}

// escalateTicket notifies the current assignee/group and the policy's
// escalation group/user, and — when the policy says so — reassigns the
// ticket to the escalation group/user (functional/hierarchical escalation).
func escalateTicket(t *models.Ticket, policy *models.SlaPolicy, now time.Time) {
	if !claimTicketAlert(t.ID, "sla_escalated_at", now) {
		return
	}
	recipients := ticketResponsibles(t)
	if policy.EscalationGroupID != nil {
		recipients = append(recipients, groupMemberIDs(*policy.EscalationGroupID)...)
	}
	if policy.EscalationUserID != nil {
		recipients = append(recipients, *policy.EscalationUserID)
	}

	updates := map[string]any{}
	t.SlaEscalatedAt = &now
	sys := SystemUserID()
	if policy.EscalationReassign {
		if g := policy.EscalationGroupID; g != nil && (t.GroupID == nil || *t.GroupID != *g) {
			updates["group_id"] = *g
			t.GroupID = g
			var grp models.UserGroup
			database.DB.First(&grp, *g)
			database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: sys, EventType: "group_changed", Detail: grp.Name})
		}
		if u := policy.EscalationUserID; u != nil && (t.AssignedToID == nil || *t.AssignedToID != *u) {
			updates["assigned_to_id"] = *u
			t.AssignedToID = u
			var usr models.User
			database.DB.First(&usr, *u)
			name := usr.DisplayName
			if name == "" {
				name = usr.Username
			}
			database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: sys, EventType: "assignee_changed", Detail: name})
		}
	}
	if len(updates) > 0 {
		database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).Updates(updates)
	}
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: sys, EventType: "sla_escalated", Detail: policy.Name})
	NotifyTicketAlert(t, recipients, TicketAlertSlaBreach)
}

// ticketResponsibles returns the assignee, the owner and the assigned group's members.
func ticketResponsibles(t *models.Ticket) []uint {
	var ids []uint
	if t.AssignedToID != nil {
		ids = append(ids, *t.AssignedToID)
	}
	if t.OwnerID != nil {
		ids = append(ids, *t.OwnerID)
	}
	if t.GroupID != nil {
		ids = append(ids, groupMemberIDs(*t.GroupID)...)
	}
	return ids
}

func groupMemberIDs(groupID uint) []uint {
	var ids []uint
	database.DB.Model(&models.GroupMember{}).Where("group_id = ?", groupID).Pluck("user_id", &ids)
	return ids
}

// NotifyMajorIncident alerts the configured major-incident group plus the
// ticket's responsibles that a ticket was flagged as a major incident.
func NotifyMajorIncident(t *models.Ticket) {
	recipients := ticketResponsibles(t)
	if g := LoadHelpdeskSettings().MajorIncidentGroupID; g != nil {
		recipients = append(recipients, groupMemberIDs(*g)...)
	}
	NotifyTicketAlert(t, recipients, TicketAlertMajorIncident)
}

// NotifyTicketAlert sends a ticket.alert WebSocket message to each online
// recipient and an email to offline recipients who opted into email
// notifications. Duplicate recipient IDs are collapsed.
func NotifyTicketAlert(t *models.Ticket, recipients []uint, kind string) {
	seen := map[uint]bool{}
	var users []models.User
	for _, id := range recipients {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		var u models.User
		if database.DB.First(&u, id).Error == nil {
			users = append(users, u)
		}
	}
	if len(users) == 0 {
		return
	}
	payload := map[string]any{
		"ticket_id":   t.ID,
		"customer_id": t.CustomerID,
		"title":       t.Title,
		"kind":        kind,
	}
	subjects := map[string]string{
		TicketAlertSlaWarning:    "SLA warning",
		TicketAlertSlaBreach:     "SLA breached — ticket escalated",
		TicketAlertMajorIncident: "Major incident",
	}
	heading := subjects[kind]
	subject := fmt.Sprintf("%s: [#%d] %s", heading, t.ID, t.Title)
	plain := fmt.Sprintf("%s for ticket #%d: %s", heading, t.ID, t.Title)
	htmlContent := fmt.Sprintf(
		`<tr><td style="padding:28px 32px;font-size:15px;color:#333;line-height:1.6">`+
			`<p style="margin:0 0 8px"><strong>%s</strong></p>`+
			`<div style="padding:12px 16px;background:#f8f8f8;border-left:4px solid #b91c1c;border-radius:4px;font-size:14px;font-weight:bold;color:#333">#%d — %s</div>`+
			`</td></tr>`,
		html.EscapeString(heading), t.ID, html.EscapeString(t.Title),
	)
	email := GetEmailService()
	for _, u := range users {
		if u.GlobalRole == "customer" {
			continue
		}
		if appws.IsUserOnline(u.ID) {
			appws.BroadcastToUser(u.ID, appws.Message{Type: appws.TypeTicketAlert, Payload: payload})
			continue
		}
		if email == nil || !u.EmailNotifications || u.Email == "" {
			continue
		}
		go email.SendHTML(u.Email, subject, WrapHTML(heading, htmlContent), WrapText(heading, plain)) //nolint:errcheck
	}
}
