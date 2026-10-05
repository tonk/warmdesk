package services

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
)

// Ticket status lifecycle:
//
//	new → open → in_progress ⇄ pending → resolved → closed
//	                                   ↘ pending_close ↗
//
// "resolved" (fix delivered, auto-closes after HelpdeskSettings.ResolvedAutoCloseDays)
// and "pending_close" (closes on an explicit close_at date) both stop the SLA
// resolution clock; "pending" (waiting on the requester) pauses it.
var ValidTicketStatuses = map[string]bool{
	"new": true, "open": true, "in_progress": true, "pending": true,
	"resolved": true, "pending_close": true, "closed": true,
}

var ValidTicketTypes = map[string]bool{
	"incident": true, "problem": true, "service_request": true, "change_request": true,
}

var ValidTicketPriorities = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

// ValidImpactUrgency are the levels of the impact × urgency priority matrix.
var ValidImpactUrgency = map[string]bool{"low": true, "medium": true, "high": true}

var ValidProblemStatuses = map[string]bool{
	"investigating": true, "root_cause_identified": true, "known_error": true, "resolved": true,
}

var ValidTicketLinkTypes = map[string]bool{
	"related": true, "duplicate_of": true, "caused_by": true, "parent_of": true,
}

// IsResolvedStatus reports whether a status counts as resolved for SLA and
// reporting purposes.
func IsResolvedStatus(s string) bool {
	return s == "resolved" || s == "pending_close" || s == "closed"
}

// StatusRequiresChecklistComplete reports whether moving to status is blocked
// by unfinished checklist items.
func StatusRequiresChecklistComplete(s string) bool {
	return IsResolvedStatus(s)
}

// TicketChecklistBlocksClose returns true when the ticket has incomplete checklist items.
func TicketChecklistBlocksClose(ticketID uint) bool {
	var count int64
	database.DB.Model(&models.TicketChecklistItem{}).
		Where("ticket_id = ? AND is_completed = ?", ticketID, false).
		Count(&count)
	return count > 0
}

// TransitionOptions tunes TransitionTicketStatus.
type TransitionOptions struct {
	// CountReopen increments reopen_count when a resolved ticket goes back to
	// an open status. Off for administrative flips such as un-marking spam.
	CountReopen bool
}

// TransitionTicketStatus applies a status change to t in memory and returns
// the column updates to persist: resolved_at/closed_at stamps, the SLA pause
// clock, reopen counting and the resolution deadline shift after a pause.
// Every code path that changes a ticket's status must go through here so the
// timestamps reports and SLA evaluation rely on stay consistent.
func TransitionTicketStatus(t *models.Ticket, newStatus string, now time.Time, opts TransitionOptions) map[string]any {
	updates := map[string]any{}
	old := t.Status
	if old == newStatus {
		return updates
	}
	t.Status = newStatus
	updates["status"] = newStatus

	// Leaving "pending": bank the paused business minutes and push the
	// resolution deadline back by that amount.
	if old == "pending" && t.SlaPausedAt != nil {
		var policy *models.SlaPolicy
		if t.SlaPolicyID != nil {
			var p models.SlaPolicy
			if database.DB.First(&p, *t.SlaPolicyID).Error == nil {
				policy = &p
			}
		}
		cal := policyCalendar(policy)
		t.SlaPausedMinutes += BusinessMinutesBetween(cal, *t.SlaPausedAt, now)
		t.SlaPausedAt = nil
		updates["sla_paused_minutes"] = t.SlaPausedMinutes
		updates["sla_paused_at"] = nil
		if policy != nil && t.SlaResolutionDeadline != nil {
			created := t.CreatedAt
			if created.IsZero() {
				created = now
			}
			_, res := ComputeSlaDeadlines(policy, cal, created, t.SlaPausedMinutes)
			t.SlaResolutionDeadline = res
			updates["sla_resolution_deadline"] = res
		}
	}
	if newStatus == "pending" && t.SlaPausedAt == nil {
		t.SlaPausedAt = &now
		updates["sla_paused_at"] = now
	}

	wasResolved, isResolved := IsResolvedStatus(old), IsResolvedStatus(newStatus)
	switch {
	case isResolved:
		if t.ResolvedAt == nil {
			t.ResolvedAt = &now
			updates["resolved_at"] = now
		}
		if newStatus == "closed" {
			if t.ClosedAt == nil {
				t.ClosedAt = &now
				updates["closed_at"] = now
			}
		} else if t.ClosedAt != nil {
			t.ClosedAt = nil
			updates["closed_at"] = nil
		}
	case wasResolved:
		t.ResolvedAt, t.ClosedAt = nil, nil
		updates["resolved_at"] = nil
		updates["closed_at"] = nil
		if opts.CountReopen {
			t.ReopenCount++
			updates["reopen_count"] = t.ReopenCount
		}
	}
	return updates
}

// SaveTicketStatus transitions a ticket's status and persists it, recording a
// status_changed history entry attributed to userID.
func SaveTicketStatus(t *models.Ticket, newStatus string, userID uint, opts TransitionOptions) {
	if t.Status == newStatus {
		return
	}
	now := time.Now()
	updates := TransitionTicketStatus(t, newStatus, now, opts)
	database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).Updates(updates)
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "status_changed", Detail: newStatus})
	RefreshSlaBreaches(t, now)
}

// ── Helpdesk settings ──────────────────────────────────────────────────────

const (
	SettingHelpdeskPriorityMatrix     = "helpdesk_priority_matrix"
	SettingHelpdeskResolvedAutoClose  = "helpdesk_resolved_autoclose_days"
	SettingHelpdeskRequireResolution  = "helpdesk_require_resolution_code"
	SettingHelpdeskMajorIncidentGroup = "helpdesk_major_incident_group_id"
)

// DefaultPriorityMatrix is the classic ITIL 3×3 impact × urgency table,
// keyed "impact:urgency".
var DefaultPriorityMatrix = map[string]string{
	"high:high": "critical", "high:medium": "high", "high:low": "medium",
	"medium:high": "high", "medium:medium": "medium", "medium:low": "low",
	"low:high": "medium", "low:medium": "low", "low:low": "low",
}

// HelpdeskSettings are the ITIL-related helpdesk options stored in system_settings.
type HelpdeskSettings struct {
	PriorityMatrix        map[string]string `json:"priority_matrix"`
	ResolvedAutoCloseDays int               `json:"resolved_autoclose_days"`
	RequireResolutionCode bool              `json:"require_resolution_code"`
	MajorIncidentGroupID  *uint             `json:"major_incident_group_id"`
}

// LoadHelpdeskSettings reads the helpdesk settings, filling in defaults.
func LoadHelpdeskSettings() HelpdeskSettings {
	s := HelpdeskSettings{ResolvedAutoCloseDays: 5}
	// "key" is a reserved word in MySQL, so filter in Go rather than in SQL.
	var rows []models.SystemSetting
	database.DB.Find(&rows)
	for _, r := range rows {
		switch r.Key {
		case SettingHelpdeskPriorityMatrix:
			var m map[string]string
			if json.Unmarshal([]byte(r.Value), &m) == nil {
				s.PriorityMatrix = m
			}
		case SettingHelpdeskResolvedAutoClose:
			if n, err := strconv.Atoi(r.Value); err == nil && n >= 0 {
				s.ResolvedAutoCloseDays = n
			}
		case SettingHelpdeskRequireResolution:
			s.RequireResolutionCode = r.Value == "true"
		case SettingHelpdeskMajorIncidentGroup:
			if n, err := strconv.ParseUint(r.Value, 10, 64); err == nil && n > 0 {
				id := uint(n)
				s.MajorIncidentGroupID = &id
			}
		}
	}
	matrix := map[string]string{}
	for k, v := range DefaultPriorityMatrix {
		matrix[k] = v
	}
	for k, v := range s.PriorityMatrix {
		if _, ok := DefaultPriorityMatrix[k]; ok && ValidTicketPriorities[v] {
			matrix[k] = v
		}
	}
	s.PriorityMatrix = matrix
	return s
}

// DerivePriority looks up the priority for an impact/urgency pair; ok is
// false when either is unset or unknown.
func DerivePriority(matrix map[string]string, impact, urgency string) (string, bool) {
	if !ValidImpactUrgency[impact] || !ValidImpactUrgency[urgency] {
		return "", false
	}
	p, ok := matrix[impact+":"+urgency]
	if !ok {
		p, ok = DefaultPriorityMatrix[impact+":"+urgency]
	}
	return p, ok
}

// AutoCloseTickets closes pending_close tickets whose close_at has passed and
// resolved tickets older than the configured auto-close period. Tickets with
// open checklist items are left alone.
func AutoCloseTickets() {
	now := time.Now()
	var due []models.Ticket
	database.DB.Where("status = 'pending_close' AND close_at IS NOT NULL AND close_at <= ?", now).Find(&due)
	if days := LoadHelpdeskSettings().ResolvedAutoCloseDays; days > 0 {
		var resolved []models.Ticket
		database.DB.Where("status = 'resolved' AND resolved_at IS NOT NULL AND resolved_at <= ?", now.AddDate(0, 0, -days)).Find(&resolved)
		due = append(due, resolved...)
	}
	for i := range due {
		if TicketChecklistBlocksClose(due[i].ID) {
			continue
		}
		updates := TransitionTicketStatus(&due[i], "closed", now, TransitionOptions{})
		database.DB.Model(&models.Ticket{}).Where("id = ?", due[i].ID).Updates(updates)
		database.DB.Create(&models.TicketHistory{TicketID: due[i].ID, UserID: SystemUserID(), EventType: "status_changed", Detail: "closed"})
	}
	// Clear close_at on already-closed tickets that still have it set
	database.DB.Model(&models.Ticket{}).
		Where("status = 'closed' AND close_at IS NOT NULL").
		Update("close_at", nil)
}

// SystemUserID returns the ID of the first admin user, or 1 as fallback — the
// author of automated ticket changes (IMAP replies, auto-close, escalation).
func SystemUserID() uint {
	var u models.User
	if err := database.DB.Where("global_role = ?", "admin").Order("id asc").First(&u).Error; err == nil {
		return u.ID
	}
	return 1
}

// BackfillTicketTimestamps fills resolved_at/closed_at on tickets resolved
// before those columns existed, from their status history (falling back to
// updated_at), so resolution metrics and breach evaluation cover old tickets.
// It only touches rows where the columns are still NULL, so it is idempotent.
func BackfillTicketTimestamps() {
	var tickets []models.Ticket
	database.DB.Select("id", "status", "updated_at", "resolved_at", "closed_at").
		Where("status IN ? AND (resolved_at IS NULL OR (status = 'closed' AND closed_at IS NULL))",
			[]string{"resolved", "pending_close", "closed"}).
		Find(&tickets)
	for _, t := range tickets {
		var hist []models.TicketHistory
		database.DB.Where("ticket_id = ? AND event_type = 'status_changed'", t.ID).Order("created_at asc").Find(&hist)
		var resolvedAt, closedAt *time.Time
		for i := range hist {
			at := hist[i].CreatedAt
			if IsResolvedStatus(hist[i].Detail) {
				if resolvedAt == nil {
					resolvedAt = &at
				}
				if hist[i].Detail == "closed" && closedAt == nil {
					closedAt = &at
				}
			} else {
				resolvedAt, closedAt = nil, nil
			}
		}
		if resolvedAt == nil {
			u := t.UpdatedAt
			resolvedAt = &u
		}
		updates := map[string]any{}
		if t.ResolvedAt == nil {
			updates["resolved_at"] = *resolvedAt
		}
		if t.Status == "closed" && t.ClosedAt == nil {
			if closedAt == nil {
				closedAt = resolvedAt
			}
			updates["closed_at"] = *closedAt
		}
		if len(updates) > 0 {
			database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).UpdateColumns(updates)
		}
	}
	if len(tickets) > 0 {
		log.Printf("helpdesk: backfilled resolution timestamps on %d ticket(s)", len(tickets))
	}
}

// SeedResolutionCodes creates a default set of resolution codes on first run.
func SeedResolutionCodes() {
	var count int64
	database.DB.Model(&models.TicketResolutionCode{}).Count(&count)
	if count > 0 {
		return
	}
	for i, name := range []string{"Fixed", "Workaround provided", "Duplicate", "User error", "No fault found", "Cancelled"} {
		database.DB.Create(&models.TicketResolutionCode{Name: name, IsActive: true, SortOrder: i})
	}
}
