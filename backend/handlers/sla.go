package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

// slaPolicyRequest is shared by create and update. Pointer fields left out of
// the JSON body are left unchanged on update; for the optional foreign keys
// (customer, category, calendar, escalation group/user) a value of 0 clears.
type slaPolicyRequest struct {
	Name                  *string `json:"name"`
	ResponseTimeMinutes   *int    `json:"response_time_minutes"`
	ResolutionTimeMinutes *int    `json:"resolution_time_minutes"`
	PriorityFilter        *string `json:"priority_filter"`
	TypeFilter            *string `json:"type_filter"`
	CustomerID            *uint   `json:"customer_id"`
	CategoryID            *uint   `json:"category_id"`
	CalendarID            *uint   `json:"calendar_id"`
	WarningPercent        *int    `json:"warning_percent"`
	EscalationGroupID     *uint   `json:"escalation_group_id"`
	EscalationUserID      *uint   `json:"escalation_user_id"`
	EscalationReassign    *bool   `json:"escalation_reassign"`
	IsActive              *bool   `json:"is_active"`
}

// cleanCSV keeps only the allowed, de-duplicated values of a comma-separated filter.
func cleanCSV(list string, allowed map[string]bool) string {
	seen := map[string]bool{}
	var out []string
	for _, v := range strings.Split(list, ",") {
		v = strings.TrimSpace(v)
		if allowed[v] && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return strings.Join(out, ",")
}

func nullableID(id *uint) any {
	if id == nil || *id == 0 {
		return nil
	}
	return *id
}

// apply validates the request and returns the column updates; errMsg is set
// when the request is invalid.
func (r *slaPolicyRequest) apply() (updates map[string]any, errMsg string) {
	updates = map[string]any{}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return nil, "name is required"
		}
		updates["name"] = name
	}
	if r.ResponseTimeMinutes != nil {
		if *r.ResponseTimeMinutes < 0 {
			return nil, "response_time_minutes must not be negative"
		}
		updates["response_time_minutes"] = *r.ResponseTimeMinutes
	}
	if r.ResolutionTimeMinutes != nil {
		if *r.ResolutionTimeMinutes < 0 {
			return nil, "resolution_time_minutes must not be negative"
		}
		updates["resolution_time_minutes"] = *r.ResolutionTimeMinutes
	}
	if r.PriorityFilter != nil {
		updates["priority_filter"] = cleanCSV(*r.PriorityFilter, services.ValidTicketPriorities)
	}
	if r.TypeFilter != nil {
		updates["type_filter"] = cleanCSV(*r.TypeFilter, services.ValidTicketTypes)
	}
	if r.WarningPercent != nil {
		if *r.WarningPercent < 0 || *r.WarningPercent > 100 {
			return nil, "warning_percent must be between 0 and 100"
		}
		updates["warning_percent"] = *r.WarningPercent
	}
	checks := []struct {
		id    *uint
		col   string
		model any
	}{
		{r.CustomerID, "customer_id", &models.Customer{}},
		{r.CategoryID, "category_id", &models.TicketCategory{}},
		{r.CalendarID, "calendar_id", &models.BusinessCalendar{}},
		{r.EscalationGroupID, "escalation_group_id", &models.UserGroup{}},
		{r.EscalationUserID, "escalation_user_id", &models.User{}},
	}
	for _, ch := range checks {
		if ch.id == nil {
			continue
		}
		if *ch.id != 0 && database.DB.First(ch.model, *ch.id).Error != nil {
			return nil, ch.col + " not found"
		}
		updates[ch.col] = nullableID(ch.id)
	}
	if r.EscalationReassign != nil {
		updates["escalation_reassign"] = *r.EscalationReassign
	}
	if r.IsActive != nil {
		updates["is_active"] = *r.IsActive
	}
	return updates, ""
}

// AdminListSlaPolicies GET /api/v1/admin/sla-policies
func AdminListSlaPolicies(c *gin.Context) {
	var policies []models.SlaPolicy
	database.DB.Order("name asc").Find(&policies)
	if policies == nil {
		policies = []models.SlaPolicy{}
	}
	c.JSON(http.StatusOK, policies)
}

// AdminCreateSlaPolicy POST /api/v1/admin/sla-policies
func AdminCreateSlaPolicy(c *gin.Context) {
	var req slaPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	updates, errMsg := req.apply()
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	policy := models.SlaPolicy{Name: updates["name"].(string), IsActive: true, WarningPercent: 75, EscalationReassign: true}
	if err := database.DB.Create(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create sla policy"})
		return
	}
	// Columns with GORM defaults (is_active, warning_percent, escalation_reassign)
	// ignore zero values on Create, so the rest goes through an explicit update.
	database.DB.Model(&policy).Updates(updates)
	database.DB.First(&policy, policy.ID)
	c.JSON(http.StatusCreated, policy)
}

// AdminUpdateSlaPolicy PUT /api/v1/admin/sla-policies/:id
func AdminUpdateSlaPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var policy models.SlaPolicy
	if err := database.DB.First(&policy, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sla policy not found"})
		return
	}

	var req slaPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updates, errMsg := req.apply()
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	if len(updates) > 0 {
		database.DB.Model(&policy).Updates(updates)
	}
	database.DB.First(&policy, id)
	c.JSON(http.StatusOK, policy)
}

// AdminDeleteSlaPolicy DELETE /api/v1/admin/sla-policies/:id
func AdminDeleteSlaPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var policy models.SlaPolicy
	if err := database.DB.First(&policy, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sla policy not found"})
		return
	}

	database.DB.Delete(&policy)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
