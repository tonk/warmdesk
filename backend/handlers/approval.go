package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

// approvalEntry is a pending approval as shown to an approver. Approvers
// don't necessarily have access to the ticket's customer, so it carries the
// context needed to decide.
type approvalEntry struct {
	TicketID           uint                    `json:"ticket_id"`
	CustomerID         *uint                   `json:"customer_id"`
	CustomerName       string                  `json:"customer_name"`
	Title              string                  `json:"title"`
	Description        string                  `json:"description"`
	Type               string                  `json:"type"`
	Priority           string                  `json:"priority"`
	ChangeType         string                  `json:"change_type,omitempty"`
	Risk               string                  `json:"risk,omitempty"`
	PlannedStart       *time.Time              `json:"planned_start,omitempty"`
	PlannedEnd         *time.Time              `json:"planned_end,omitempty"`
	ImplementationPlan string                  `json:"implementation_plan,omitempty"`
	BackoutPlan        string                  `json:"backout_plan,omitempty"`
	RequestData        models.RequestData      `json:"request_data,omitempty"`
	CatalogItemName    string                  `json:"catalog_item_name,omitempty"`
	ApprovalsRequired  int                     `json:"approvals_required"`
	ApprovalRound      int                     `json:"approval_round"`
	Approved           int                     `json:"approved"`
	Rejected           int                     `json:"rejected"`
	MyDecision         string                  `json:"my_decision"`
	RequestedBy        string                  `json:"requested_by"`
	CreatedAt          time.Time               `json:"created_at"`
	Decisions          []models.TicketApproval `json:"decisions"`
}

func buildApprovalEntry(t *models.Ticket, userID uint) approvalEntry {
	e := approvalEntry{
		TicketID: t.ID, CustomerID: t.CustomerID, Title: t.Title, Description: t.Description,
		Type: t.Type, Priority: t.Priority, ChangeType: t.ChangeType, Risk: t.Risk,
		PlannedStart: t.PlannedStart, PlannedEnd: t.PlannedEnd,
		ImplementationPlan: t.ImplementationPlan, BackoutPlan: t.BackoutPlan,
		RequestData: t.RequestData, ApprovalsRequired: t.ApprovalsRequired,
		ApprovalRound: t.ApprovalRound, CreatedAt: t.CreatedAt,
	}
	if t.Customer != nil {
		e.CustomerName = t.Customer.Name
	}
	e.RequestedBy = t.CreatedBy.DisplayName
	if e.RequestedBy == "" {
		e.RequestedBy = t.CreatedBy.Username
	}
	if t.CatalogItemID != nil {
		var item models.ServiceCatalogItem
		if database.DB.Select("name").First(&item, *t.CatalogItemID).Error == nil {
			e.CatalogItemName = item.Name
		}
	}
	database.DB.Preload("User").Where("ticket_id = ? AND round = ?", t.ID, t.ApprovalRound).Order("created_at asc").Find(&e.Decisions)
	if e.Decisions == nil {
		e.Decisions = []models.TicketApproval{}
	}
	for _, d := range e.Decisions {
		switch d.Decision {
		case services.ApprovalApproved:
			e.Approved++
		case services.ApprovalRejected:
			e.Rejected++
		}
		if d.UserID == userID {
			e.MyDecision = d.Decision
		}
	}
	return e
}

// ListMyApprovals GET /api/v1/approvals — pending approval rounds the caller
// can decide on (members of the approval group; admins see all).
func ListMyApprovals(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if requireNotCustomerRole(role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	q := database.DB.Where("approval_status = ?", services.ApprovalPending)
	if role != "admin" {
		var groupIDs []uint
		database.DB.Model(&models.GroupMember{}).Where("user_id = ?", userID).Pluck("group_id", &groupIDs)
		if len(groupIDs) == 0 {
			c.JSON(http.StatusOK, []approvalEntry{})
			return
		}
		q = q.Where("approval_group_id IN ?", groupIDs)
	}
	var tickets []models.Ticket
	q.Preload("Customer").Preload("CreatedBy").Order("created_at asc").Find(&tickets)
	out := make([]approvalEntry, 0, len(tickets))
	for i := range tickets {
		out = append(out, buildApprovalEntry(&tickets[i], userID))
	}
	c.JSON(http.StatusOK, out)
}

// DecideApproval POST /api/v1/approvals/:ticketId  {"decision": "approved"|"rejected", "comment": "…"}
func DecideApproval(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if requireNotCustomerRole(role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, ok := paramID(c, "ticketId")
	if !ok {
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var t models.Ticket
	if database.DB.First(&t, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}
	if req.Decision == services.ApprovalRejected && strings.TrimSpace(req.Comment) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "give a reason when rejecting", "code": "comment_required"})
		return
	}
	status, err := services.RecordApprovalDecision(&t, userID, role, req.Decision, strings.TrimSpace(req.Comment))
	switch {
	case errors.Is(err, services.ErrNotAnApprover):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "not_an_approver"})
		return
	case errors.Is(err, services.ErrNoApprovalPending):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "no_approval_pending"})
		return
	case err != nil:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Preload("Customer").Preload("CreatedBy").First(&t, t.ID)
	c.JSON(http.StatusOK, gin.H{"approval_status": status, "approval": buildApprovalEntry(&t, userID)})
}

// GetTicketApprovals GET /customers/:customerId/tickets/:ticketId/approvals —
// every decision on the ticket, all rounds, newest round first.
func GetTicketApprovals(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, false)
	if !ok {
		return
	}
	var rows []models.TicketApproval
	database.DB.Preload("User").Where("ticket_id = ?", t.ID).Order("round desc, created_at asc").Find(&rows)
	if rows == nil {
		rows = []models.TicketApproval{}
	}
	approved, rejected := services.ApprovalTally(t)
	c.JSON(http.StatusOK, gin.H{
		"approval_status":    t.ApprovalStatus,
		"approval_group_id":  t.ApprovalGroupID,
		"approvals_required": t.ApprovalsRequired,
		"approval_round":     t.ApprovalRound,
		"approved":           approved,
		"rejected":           rejected,
		"can_decide":         t.ApprovalStatus == services.ApprovalPending && services.IsApprover(t, userID, role),
		"decisions":          rows,
	})
}

// RequestTicketApproval POST /customers/:customerId/tickets/:ticketId/approvals/request
// {"group_id": 3, "approvals_required": 2} — starts a (new) approval round on
// any non-change ticket; changes request approval through their change status.
func RequestTicketApproval(c *gin.Context) {
	t, userID, _, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	if t.Type == "change_request" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "submit the change for approval instead", "code": "use_change_submit"})
		return
	}
	var req struct {
		GroupID           uint `json:"group_id"`
		ApprovalsRequired int  `json:"approvals_required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}
	if database.DB.First(&models.UserGroup{}, req.GroupID).Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
		return
	}
	updates := services.StartApprovalRound(t, req.GroupID, req.ApprovalsRequired)
	database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).Updates(updates)
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "approval_requested"})
	services.NotifyApprovers(t)
	reloaded, _ := preloadTicket(t.ID)
	c.JSON(http.StatusOK, reloaded)
}
