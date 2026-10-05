package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
)

// loadCustomerTicket resolves /customers/:customerId/tickets/:ticketId for
// the caller: it checks customer access (and, when write is set, that the
// caller is not a customer-portal user), hides problem records from
// customer-portal users, and writes the error response itself when it
// returns ok=false.
func loadCustomerTicket(c *gin.Context, write bool) (ticket *models.Ticket, userID uint, role string, ok bool) {
	userID = middleware.GetUserID(c)
	role = middleware.GetGlobalRole(c)
	customerID, err := strconv.ParseUint(c.Param("customerId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return nil, 0, "", false
	}
	ticketID, err := strconv.ParseUint(c.Param("ticketId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return nil, 0, "", false
	}
	if requireCustomerAccess(uint(customerID), userID, role) != nil || (write && requireNotCustomerRole(role) != nil) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, 0, "", false
	}
	var t models.Ticket
	if err := database.DB.Where("id = ? AND customer_id = ?", ticketID, customerID).First(&t).Error; err != nil ||
		(role == "customer" && t.Type == "problem") {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return nil, 0, "", false
	}
	return &t, userID, role, true
}

// paramID parses a numeric route parameter, writing a 400 when it isn't one.
func paramID(c *gin.Context, name string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return 0, false
	}
	return uint(n), true
}

// canSeeCustomerScoped reports whether the caller may see a record scoped to
// customerID (nil = internal/global): admins always, agents with customer
// access, and — when allowCustomerRole — customer-portal users with access.
func canSeeCustomerScoped(customerID *uint, userID uint, role string, allowCustomerRole bool) bool {
	if role == "customer" && !allowCustomerRole {
		return false
	}
	if customerID == nil {
		return true
	}
	return requireCustomerAccess(*customerID, userID, role) == nil
}
