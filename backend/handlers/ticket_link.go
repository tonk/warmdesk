package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

type ticketLinkEntry struct {
	LinkID     uint   `json:"link_id"`
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	Type       string `json:"type"`
	CustomerID *uint  `json:"customer_id"`
	// LinkType is read as "source <link_type> target"; Direction says which
	// side the ticket being viewed is on: "outgoing" when it is the source
	// (e.g. "this ticket is caused by #12"), "incoming" when it is the target
	// ("#12 is caused by this ticket").
	LinkType  string `json:"link_type"`
	Direction string `json:"direction"`
	// Workaround is included for linked known errors so an agent can reuse it.
	Workaround   string `json:"workaround,omitempty"`
	IsKnownError bool   `json:"is_known_error"`
}

// canSeeTicket reports whether a user may see another ticket referenced by a
// link: admins see everything, agents need access to the ticket's customer
// (inbox tickets are visible to any agent), and customer-portal users never
// see problem records.
func canSeeTicket(t *models.Ticket, userID uint, role string) bool {
	if role == "customer" && t.Type == "problem" {
		return false
	}
	if t.CustomerID == nil {
		return role != "customer"
	}
	return requireCustomerAccess(*t.CustomerID, userID, role) == nil
}

func linkEntry(l models.TicketLink, viewedID uint, t models.Ticket) ticketLinkEntry {
	dir := "outgoing"
	if l.TargetTicketID == viewedID {
		dir = "incoming"
	}
	lt := l.LinkType
	if lt == "" {
		lt = "related"
	}
	e := ticketLinkEntry{
		LinkID: l.ID, ID: t.ID, Title: t.Title, Status: t.Status, Priority: t.Priority,
		Type: t.Type, CustomerID: t.CustomerID, LinkType: lt, Direction: dir,
		IsKnownError: t.IsKnownError,
	}
	if t.Type == "problem" {
		e.Workaround = t.Workaround
	}
	return e
}

// ListTicketLinks GET /api/v1/customers/:customerId/tickets/:ticketId/links
func ListTicketLinks(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	customerID, err := strconv.ParseUint(c.Param("customerId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	ticketID, err := strconv.ParseUint(c.Param("ticketId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	if err := requireCustomerAccess(uint(customerID), userID, role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var ticket models.Ticket
	if err := database.DB.Select("id", "type").Where("id = ? AND customer_id = ?", ticketID, customerID).First(&ticket).Error; err != nil ||
		(role == "customer" && ticket.Type == "problem") {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	c.JSON(http.StatusOK, loadTicketLinks(uint(ticketID), userID, role))
}

// loadTicketLinks returns the links of a ticket, leaving out linked tickets
// the user isn't allowed to see.
func loadTicketLinks(ticketID, userID uint, role string) []ticketLinkEntry {
	var links []models.TicketLink
	database.DB.Where("source_ticket_id = ? OR target_ticket_id = ?", ticketID, ticketID).Order("id asc").Find(&links)

	out := make([]ticketLinkEntry, 0, len(links))
	for _, l := range links {
		linkedID := l.TargetTicketID
		if l.TargetTicketID == ticketID {
			linkedID = l.SourceTicketID
		}
		var t models.Ticket
		if err := database.DB.Select("id", "title", "status", "priority", "type", "customer_id", "workaround", "is_known_error").First(&t, linkedID).Error; err != nil {
			continue
		}
		if !canSeeTicket(&t, userID, role) {
			continue
		}
		out = append(out, linkEntry(l, ticketID, t))
	}
	return out
}

// CreateTicketLink POST /api/v1/customers/:customerId/tickets/:ticketId/links
//
// Body: {"target_ticket_id": 12, "link_type": "caused_by"}. link_type defaults
// to "related"; directional types read "this ticket <link_type> target".
// The target may belong to another customer (e.g. a cross-customer problem)
// as long as the caller has access to it.
func CreateTicketLink(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	customerID, err := strconv.ParseUint(c.Param("customerId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	ticketID, err := strconv.ParseUint(c.Param("ticketId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	if err := requireCustomerAccess(uint(customerID), userID, role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := requireNotCustomerRole(role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var ticket models.Ticket
	if err := database.DB.Where("id = ? AND customer_id = ?", ticketID, customerID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	var req struct {
		TargetTicketID uint   `json:"target_ticket_id" binding:"required"`
		LinkType       string `json:"link_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_ticket_id is required"})
		return
	}
	if req.LinkType == "" {
		req.LinkType = "related"
	}
	if !services.ValidTicketLinkTypes[req.LinkType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link_type"})
		return
	}

	if req.TargetTicketID == uint(ticketID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot link ticket to itself"})
		return
	}

	var target models.Ticket
	if err := database.DB.First(&target, req.TargetTicketID).Error; err != nil || !canSeeTicket(&target, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "target ticket not found"})
		return
	}

	// Prevent duplicate links (check both directions)
	var count int64
	database.DB.Model(&models.TicketLink{}).
		Where("(source_ticket_id = ? AND target_ticket_id = ?) OR (source_ticket_id = ? AND target_ticket_id = ?)",
			ticketID, req.TargetTicketID, req.TargetTicketID, ticketID).
		Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "tickets are already linked"})
		return
	}

	link := models.TicketLink{
		SourceTicketID: uint(ticketID),
		TargetTicketID: req.TargetTicketID,
		LinkType:       req.LinkType,
	}
	if err := database.DB.Create(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	database.DB.Create(&models.TicketHistory{TicketID: uint(ticketID), UserID: userID, EventType: "ticket_linked", Detail: req.LinkType + " #" + strconv.FormatUint(uint64(target.ID), 10) + " " + target.Title})
	database.DB.Create(&models.TicketHistory{TicketID: target.ID, UserID: userID, EventType: "ticket_linked", Detail: req.LinkType + " ← #" + strconv.FormatUint(ticketID, 10) + " " + ticket.Title})
	c.JSON(http.StatusCreated, linkEntry(link, uint(ticketID), target))
}

// DeleteTicketLink DELETE /api/v1/customers/:customerId/tickets/:ticketId/links/:linkId
func DeleteTicketLink(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	customerID, err := strconv.ParseUint(c.Param("customerId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}
	ticketID, err := strconv.ParseUint(c.Param("ticketId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	linkID, err := strconv.ParseUint(c.Param("linkId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link id"})
		return
	}
	if err := requireCustomerAccess(uint(customerID), userID, role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := requireNotCustomerRole(role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var ticket models.Ticket
	if err := database.DB.Select("id").Where("id = ? AND customer_id = ?", ticketID, customerID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	database.DB.Where("id = ? AND (source_ticket_id = ? OR target_ticket_id = ?)", linkID, ticketID, ticketID).Delete(&models.TicketLink{})
	database.DB.Create(&models.TicketHistory{TicketID: uint(ticketID), UserID: userID, EventType: "ticket_unlinked"})
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}
