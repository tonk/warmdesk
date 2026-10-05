package services

import (
	"errors"
	"strconv"
	"time"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"gorm.io/gorm/clause"
)

// Approval statuses of a ticket's approval round.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// Ticket alert kinds for approvals (see NotifyTicketAlert).
const (
	TicketAlertApprovalRequested = "approval_requested"
	TicketAlertApprovalApproved  = "approval_approved"
	TicketAlertApprovalRejected  = "approval_rejected"
)

var (
	ErrNoApprovalPending = errors.New("no approval is pending on this ticket")
	ErrNotAnApprover     = errors.New("you are not an approver for this ticket")
	ErrInvalidDecision   = errors.New("decision must be approved or rejected")
	ErrNoApproverGroup   = errors.New("no approver group is configured")
)

// ApprovalBlocksStatus reports whether a ticket's approval state forbids
// moving to status: fulfilment (in_progress) and resolution may only happen
// once the approval round is approved. Closing (e.g. a rejected request)
// stays possible.
func ApprovalBlocksStatus(t *models.Ticket, status string) bool {
	if t.ApprovalStatus != ApprovalPending && t.ApprovalStatus != ApprovalRejected {
		return false
	}
	return status == "in_progress" || status == "resolved" || status == "pending_close"
}

// IsApprover reports whether userID may decide on the ticket's approval:
// members of the approval group, plus global admins as an override.
func IsApprover(t *models.Ticket, userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	if t.ApprovalGroupID == nil {
		return false
	}
	var n int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", *t.ApprovalGroupID, userID).Count(&n)
	return n > 0
}

// StartApprovalRound opens a new approval round on t (in memory) and returns
// the column updates. required < 1 is treated as 1.
func StartApprovalRound(t *models.Ticket, groupID uint, required int) map[string]any {
	if required < 1 {
		required = 1
	}
	t.ApprovalStatus = ApprovalPending
	t.ApprovalGroupID = &groupID
	t.ApprovalsRequired = required
	t.ApprovalRound++
	return map[string]any{
		"approval_status":    ApprovalPending,
		"approval_group_id":  groupID,
		"approvals_required": required,
		"approval_round":     t.ApprovalRound,
	}
}

// NotifyApprovers alerts the members of the ticket's approval group.
func NotifyApprovers(t *models.Ticket) {
	if t.ApprovalGroupID == nil {
		return
	}
	NotifyTicketAlert(t, groupMemberIDs(*t.ApprovalGroupID), TicketAlertApprovalRequested)
}

// ApprovalTally counts the decisions of the ticket's current round.
func ApprovalTally(t *models.Ticket) (approved, rejected int) {
	var rows []models.TicketApproval
	database.DB.Where("ticket_id = ? AND round = ?", t.ID, t.ApprovalRound).Find(&rows)
	for _, r := range rows {
		switch r.Decision {
		case ApprovalApproved:
			approved++
		case ApprovalRejected:
			rejected++
		}
	}
	return
}

// RecordApprovalDecision stores userID's decision on the current round
// (replacing an earlier decision of the same user in that round) and settles
// the round: any rejection rejects it, ApprovalsRequired approvals approve
// it. It returns the resulting approval status.
func RecordApprovalDecision(t *models.Ticket, userID uint, role, decision, comment string) (string, error) {
	if decision != ApprovalApproved && decision != ApprovalRejected {
		return "", ErrInvalidDecision
	}
	if t.ApprovalStatus != ApprovalPending {
		return "", ErrNoApprovalPending
	}
	if !IsApprover(t, userID, role) {
		return "", ErrNotAnApprover
	}
	now := time.Now()
	row := models.TicketApproval{TicketID: t.ID, Round: t.ApprovalRound, UserID: userID, Decision: decision, Comment: comment}
	database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ticket_id"}, {Name: "round"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{"decision": decision, "comment": comment, "updated_at": now}),
	}).Create(&row)
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "approval_" + decision, Detail: comment})

	approved, rejected := ApprovalTally(t)
	switch {
	case rejected > 0:
		settleApproval(t, ApprovalRejected, now)
	case approved >= t.ApprovalsRequired:
		settleApproval(t, ApprovalApproved, now)
	}
	return t.ApprovalStatus, nil
}

// settleApproval closes the round, advances an awaiting change, records
// history and tells the people working the ticket (and its creator).
func settleApproval(t *models.Ticket, status string, now time.Time) {
	t.ApprovalStatus = status
	updates := map[string]any{"approval_status": status}
	if t.Type == "change_request" && t.ChangeStatus == "awaiting_approval" {
		if status == ApprovalApproved {
			t.ChangeStatus = "approved"
		} else {
			t.ChangeStatus = "draft"
		}
		updates["change_status"] = t.ChangeStatus
	}
	database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).Updates(updates)
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: SystemUserID(), EventType: "approval_round_" + status,
		Detail: "round " + strconv.Itoa(t.ApprovalRound)})
	kind := TicketAlertApprovalApproved
	if status == ApprovalRejected {
		kind = TicketAlertApprovalRejected
	}
	recipients := append(ticketResponsibles(t), t.CreatedByID)
	NotifyTicketAlert(t, recipients, kind)
}
