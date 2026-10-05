package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
)

// Change lifecycle (Type == "change_request"):
//
//	draft → awaiting_approval → approved → scheduled → implementing → review → completed | failed
//	  ↑            │ rejected                  │ unschedule
//	  └────────────┘                           ↓
//	                                       approved          (any open state) → cancelled
//
// Standard changes are pre-approved: submitting one goes straight to approved.
var ValidChangeStatuses = map[string]bool{
	"draft": true, "awaiting_approval": true, "approved": true, "scheduled": true,
	"implementing": true, "review": true, "completed": true, "failed": true, "cancelled": true,
}

var ValidChangeTypes = map[string]bool{"standard": true, "normal": true, "emergency": true}
var ValidRiskLevels = map[string]bool{"low": true, "medium": true, "high": true}
var ValidChangeOutcomes = map[string]bool{"successful": true, "partially_successful": true, "failed": true}

// changeTransitions lists the allowed next statuses per status.
var changeTransitions = map[string][]string{
	"draft":             {"awaiting_approval", "cancelled"},
	"awaiting_approval": {"draft", "cancelled"},
	"approved":          {"scheduled", "draft", "cancelled"},
	"scheduled":         {"implementing", "approved", "cancelled"},
	"implementing":      {"review", "failed"},
	"review":            {"completed", "failed"},
}

// ChangeError is a rejected change transition; Code is a stable machine
// readable reason for the UI.
type ChangeError struct {
	Code string
	Msg  string
}

func (e *ChangeError) Error() string { return e.Msg }

func changeErr(code, msg string) *ChangeError { return &ChangeError{code, msg} }

// ChangeSettings are the change-management options stored in system_settings.
type ChangeSettings struct {
	CABGroupID                 *uint `json:"cab_group_id"`
	ApprovalsRequired          int   `json:"approvals_required"`
	EmergencyApprovalsRequired int   `json:"emergency_approvals_required"`
}

// FreezeConflict / ScheduleConflict are reported by CheckChangeSchedule.
type FreezeConflict struct {
	ID    uint      `json:"id"`
	Name  string    `json:"name"`
	Start time.Time `json:"start_at"`
	End   time.Time `json:"end_at"`
}

type ScheduleConflict struct {
	TicketID   uint      `json:"ticket_id"`
	CustomerID *uint     `json:"customer_id"`
	Title      string    `json:"title"`
	Start      time.Time `json:"planned_start"`
	End        time.Time `json:"planned_end"`
	CIs        []string  `json:"cis"`
}

type ScheduleCheck struct {
	Freezes   []FreezeConflict   `json:"freezes"`
	Conflicts []ScheduleConflict `json:"conflicts"`
}

// CheckChangeSchedule reports freeze windows overlapping the change's planned
// window (global ones and the change's customer's) and other open changes in
// an overlapping window that touch one of the same configuration items.
func CheckChangeSchedule(t *models.Ticket) ScheduleCheck {
	out := ScheduleCheck{Freezes: []FreezeConflict{}, Conflicts: []ScheduleConflict{}}
	if t.PlannedStart == nil || t.PlannedEnd == nil {
		return out
	}
	start, end := *t.PlannedStart, *t.PlannedEnd
	var freezes []models.ChangeFreezeWindow
	q := database.DB.Where("start_at < ? AND end_at > ?", end, start)
	if t.CustomerID != nil {
		q = q.Where("customer_id IS NULL OR customer_id = ?", *t.CustomerID)
	} else {
		q = q.Where("customer_id IS NULL")
	}
	q.Order("start_at asc").Find(&freezes)
	for _, f := range freezes {
		out.Freezes = append(out.Freezes, FreezeConflict{f.ID, f.Name, f.StartAt, f.EndAt})
	}

	var ciIDs []uint
	database.DB.Model(&models.TicketCI{}).Where("ticket_id = ?", t.ID).Pluck("ci_id", &ciIDs)
	if len(ciIDs) == 0 {
		return out
	}
	var others []models.Ticket
	database.DB.Where("id <> ? AND type = ? AND planned_start < ? AND planned_end > ? AND change_status IN ?",
		t.ID, "change_request", end, start, []string{"approved", "scheduled", "implementing"}).Find(&others)
	for _, o := range others {
		var shared []string
		database.DB.Model(&models.ConfigurationItem{}).
			Joins("JOIN ticket_cis ON ticket_cis.ci_id = configuration_items.id").
			Where("ticket_cis.ticket_id = ? AND configuration_items.id IN ?", o.ID, ciIDs).
			Pluck("configuration_items.name", &shared)
		if len(shared) > 0 {
			out.Conflicts = append(out.Conflicts, ScheduleConflict{o.ID, o.CustomerID, o.Title, *o.PlannedStart, *o.PlannedEnd, shared})
		}
	}
	return out
}

// TransitionChange validates moving a change to status and applies it to t
// in memory, returning the column updates. Submitting (→ awaiting_approval)
// opens an approval round with the CAB — or approves a standard change
// directly; scheduling requires a planned window outside freeze windows
// (emergency changes may override a freeze); completing/failing requires an
// outcome and resolves the ticket; cancelling closes it.
func TransitionChange(t *models.Ticket, to string, now time.Time) (map[string]any, error) {
	if t.Type != "change_request" {
		return nil, changeErr("not_a_change", "only change requests have a change status")
	}
	from := t.ChangeStatus
	if from == "" {
		from = "draft"
	}
	if from == to {
		return map[string]any{}, nil
	}
	if !ValidChangeStatuses[to] {
		return nil, changeErr("invalid_change_status", "invalid change status")
	}
	allowed := false
	for _, s := range changeTransitions[from] {
		allowed = allowed || s == to
	}
	if !allowed {
		return nil, changeErr("invalid_change_transition", fmt.Sprintf("a change can't go from %s to %s", from, to))
	}

	updates := map[string]any{}
	settings := LoadChangeSettings()
	switch to {
	case "awaiting_approval":
		var missing []string
		if strings.TrimSpace(t.ImplementationPlan) == "" {
			missing = append(missing, "implementation plan")
		}
		if strings.TrimSpace(t.BackoutPlan) == "" {
			missing = append(missing, "backout plan")
		}
		if t.Risk == "" {
			missing = append(missing, "risk")
		}
		if len(missing) > 0 && t.ChangeType != "standard" {
			return nil, changeErr("change_incomplete", "fill in the "+strings.Join(missing, ", ")+" before submitting")
		}
		if t.ChangeType == "standard" {
			// Pre-approved: no CAB round.
			to = "approved"
			t.ApprovalStatus = ApprovalApproved
			updates["approval_status"] = ApprovalApproved
			break
		}
		if settings.CABGroupID == nil {
			return nil, changeErr("no_cab_group", "no change advisory board group is configured")
		}
		required := settings.ApprovalsRequired
		if t.ChangeType == "emergency" {
			required = settings.EmergencyApprovalsRequired
		}
		for k, v := range StartApprovalRound(t, *settings.CABGroupID, required) {
			updates[k] = v
		}
	case "scheduled":
		if t.ApprovalStatus != ApprovalApproved {
			return nil, changeErr("approval_required", "the change must be approved before it is scheduled")
		}
		if t.PlannedStart == nil || t.PlannedEnd == nil || !t.PlannedEnd.After(*t.PlannedStart) {
			return nil, changeErr("planned_window_required", "set a planned start and end before scheduling")
		}
		if t.ChangeType != "emergency" && len(CheckChangeSchedule(t).Freezes) > 0 {
			return nil, changeErr("change_freeze", "the planned window overlaps a change freeze")
		}
	case "implementing":
		if t.ApprovalStatus != ApprovalApproved {
			return nil, changeErr("approval_required", "the change must be approved before it is implemented")
		}
	case "draft":
		// Back to the drawing board (withdrawn, or revising an approved
		// change): the next submission needs a fresh approval round.
		t.ApprovalStatus = ""
		updates["approval_status"] = ""
	case "completed", "failed":
		if !ValidChangeOutcomes[t.ChangeOutcome] {
			return nil, changeErr("outcome_required", "record the outcome of the change first")
		}
	}

	t.ChangeStatus = to
	updates["change_status"] = to
	switch to {
	case "completed", "failed":
		for k, v := range TransitionTicketStatus(t, "resolved", now, TransitionOptions{}) {
			updates[k] = v
		}
	case "cancelled":
		for k, v := range TransitionTicketStatus(t, "closed", now, TransitionOptions{}) {
			updates[k] = v
		}
	case "implementing":
		for k, v := range TransitionTicketStatus(t, "in_progress", now, TransitionOptions{}) {
			updates[k] = v
		}
	}
	return updates, nil
}

// LoadChangeSettings reads the change settings with defaults (1 approval).
func LoadChangeSettings() ChangeSettings {
	s := ChangeSettings{ApprovalsRequired: 1, EmergencyApprovalsRequired: 1}
	var rows []models.SystemSetting
	database.DB.Find(&rows)
	for _, r := range rows {
		switch r.Key {
		case SettingChangeCABGroup:
			if id := parseUintSetting(r.Value); id != nil {
				s.CABGroupID = id
			}
		case SettingChangeApprovals:
			if n := parseIntSetting(r.Value); n > 0 {
				s.ApprovalsRequired = n
			}
		case SettingChangeEmergencyApprovals:
			if n := parseIntSetting(r.Value); n > 0 {
				s.EmergencyApprovalsRequired = n
			}
		}
	}
	return s
}

const (
	SettingChangeCABGroup           = "change_cab_group_id"
	SettingChangeApprovals          = "change_approvals_required"
	SettingChangeEmergencyApprovals = "change_emergency_approvals_required"
)

// IsChangeError unwraps a *ChangeError.
func IsChangeError(err error) (*ChangeError, bool) {
	var ce *ChangeError
	ok := errors.As(err, &ce)
	return ce, ok
}

func parseUintSetting(v string) *uint {
	n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	if err != nil || n == 0 {
		return nil
	}
	id := uint(n)
	return &id
}

func parseIntSetting(v string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(v))
	return n
}
