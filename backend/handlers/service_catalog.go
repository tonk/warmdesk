package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

var validCatalogFieldTypes = map[string]bool{
	"text": true, "textarea": true, "select": true, "checkbox": true, "date": true, "number": true,
}

var catalogFieldKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)

// ListServiceCatalog GET /api/v1/service-catalog?customer_id=…
// Active items available to the caller; with customer_id, only the items
// orderable for that customer (global ones plus that customer's own).
// Customer-portal users can order too.
func ListServiceCatalog(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	q := database.DB.Where("is_active = ?", true)
	if v := c.Query("customer_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil || requireCustomerAccess(uint(id), userID, role) != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		q = q.Where("customer_id IS NULL OR customer_id = ?", id)
	}
	var items []models.ServiceCatalogItem
	q.Order("sort_order asc, name asc").Find(&items)
	out := make([]models.ServiceCatalogItem, 0, len(items))
	for _, it := range items {
		if canSeeCustomerScoped(it.CustomerID, userID, role, true) {
			out = append(out, it)
		}
	}
	c.JSON(http.StatusOK, out)
}

// AdminListServiceCatalog GET /api/v1/admin/service-catalog
func AdminListServiceCatalog(c *gin.Context) {
	var items []models.ServiceCatalogItem
	database.DB.Order("sort_order asc, name asc").Find(&items)
	if items == nil {
		items = []models.ServiceCatalogItem{}
	}
	c.JSON(http.StatusOK, items)
}

type catalogItemRequest struct {
	Name                *string               `json:"name"`
	Description         *string               `json:"description"`
	Icon                *string               `json:"icon"`
	CategoryID          *uint                 `json:"category_id"`
	CustomerID          *uint                 `json:"customer_id"`
	Fields              *models.CatalogFields `json:"fields"`
	DefaultPriority     *string               `json:"default_priority"`
	GroupID             *uint                 `json:"group_id"`
	ApprovalGroupID     *uint                 `json:"approval_group_id"`
	ApprovalsRequired   *int                  `json:"approvals_required"`
	ChecklistTemplateID *uint                 `json:"checklist_template_id"`
	IsActive            *bool                 `json:"is_active"`
	SortOrder           *int                  `json:"sort_order"`
}

func validateCatalogFields(fields models.CatalogFields) string {
	seen := map[string]bool{}
	for i, f := range fields {
		if !catalogFieldKeyRe.MatchString(f.Key) {
			return fmt.Sprintf("field %d: key must start with a letter and contain only a-z, 0-9 and _", i+1)
		}
		if seen[f.Key] {
			return "duplicate field key " + f.Key
		}
		seen[f.Key] = true
		if strings.TrimSpace(f.Label) == "" {
			return "field " + f.Key + ": label is required"
		}
		if !validCatalogFieldTypes[f.Type] {
			return "field " + f.Key + ": invalid type"
		}
		if f.Type == "select" && len(f.Options) == 0 {
			return "field " + f.Key + ": a select needs options"
		}
	}
	return ""
}

func (r *catalogItemRequest) updates() (map[string]any, string) {
	u := map[string]any{}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return nil, "name is required"
		}
		u["name"] = name
	}
	if r.Description != nil {
		u["description"] = *r.Description
	}
	if r.Icon != nil {
		u["icon"] = *r.Icon
	}
	if r.Fields != nil {
		if msg := validateCatalogFields(*r.Fields); msg != "" {
			return nil, msg
		}
		u["fields"] = *r.Fields
	}
	if r.DefaultPriority != nil {
		if !services.ValidTicketPriorities[*r.DefaultPriority] {
			return nil, "invalid default_priority"
		}
		u["default_priority"] = *r.DefaultPriority
	}
	if r.ApprovalsRequired != nil {
		if *r.ApprovalsRequired < 0 || *r.ApprovalsRequired > 20 {
			return nil, "approvals_required must be between 0 and 20"
		}
		u["approvals_required"] = *r.ApprovalsRequired
	}
	for _, ch := range []struct {
		id    *uint
		col   string
		model any
	}{
		{r.CategoryID, "category_id", &models.TicketCategory{}},
		{r.CustomerID, "customer_id", &models.Customer{}},
		{r.GroupID, "group_id", &models.UserGroup{}},
		{r.ApprovalGroupID, "approval_group_id", &models.UserGroup{}},
		{r.ChecklistTemplateID, "checklist_template_id", &models.TicketChecklistTemplate{}},
	} {
		if ch.id == nil {
			continue
		}
		if *ch.id != 0 && database.DB.First(ch.model, *ch.id).Error != nil {
			return nil, ch.col + " not found"
		}
		u[ch.col] = nullableID(ch.id)
	}
	if r.IsActive != nil {
		u["is_active"] = *r.IsActive
	}
	if r.SortOrder != nil {
		u["sort_order"] = *r.SortOrder
	}
	return u, ""
}

// checkCatalogApproval rejects an item that needs approvals but has no approver group.
func checkCatalogApproval(item *models.ServiceCatalogItem) string {
	if item.ApprovalsRequired > 0 && item.ApprovalGroupID == nil {
		return "an approver group is required when approvals are required"
	}
	return ""
}

// AdminCreateServiceCatalogItem POST /api/v1/admin/service-catalog
func AdminCreateServiceCatalogItem(c *gin.Context) {
	var req catalogItemRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	u, msg := req.updates()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	item := models.ServiceCatalogItem{Name: u["name"].(string), IsActive: true, DefaultPriority: "medium", Fields: models.CatalogFields{}}
	if err := database.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create catalogue item"})
		return
	}
	database.DB.Model(&item).Updates(u)
	database.DB.First(&item, item.ID)
	if msg := checkCatalogApproval(&item); msg != "" {
		database.DB.Delete(&item)
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// AdminUpdateServiceCatalogItem PUT /api/v1/admin/service-catalog/:id
func AdminUpdateServiceCatalogItem(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var item models.ServiceCatalogItem
	if database.DB.First(&item, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalogue item not found"})
		return
	}
	var req catalogItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, msg := req.updates()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	// Validate the result before persisting.
	probe := item
	if v, ok := u["approvals_required"].(int); ok {
		probe.ApprovalsRequired = v
	}
	if v, ok := u["approval_group_id"]; ok {
		if v == nil {
			probe.ApprovalGroupID = nil
		} else {
			g := v.(uint)
			probe.ApprovalGroupID = &g
		}
	}
	if msg := checkCatalogApproval(&probe); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if len(u) > 0 {
		database.DB.Model(&item).Updates(u)
	}
	database.DB.First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

// AdminDeleteServiceCatalogItem DELETE /api/v1/admin/service-catalog/:id
// Tickets ordered from the item keep their answers.
func AdminDeleteServiceCatalogItem(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	database.DB.Model(&models.Ticket{}).Where("catalog_item_id = ?", id).Update("catalog_item_id", nil)
	database.DB.Delete(&models.ServiceCatalogItem{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// validateAnswers checks request-form answers against the item's fields and
// returns the normalised answers.
func validateAnswers(fields models.CatalogFields, answers map[string]any) (models.RequestData, string) {
	out := models.RequestData{}
	for _, f := range fields {
		raw, present := answers[f.Key]
		var val any
		switch f.Type {
		case "checkbox":
			b, _ := raw.(bool)
			val = b
			if f.Required && !b {
				return nil, f.Label + " must be checked"
			}
		case "number":
			switch v := raw.(type) {
			case float64:
				val = v
			case string:
				if strings.TrimSpace(v) != "" {
					n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
					if err != nil {
						return nil, f.Label + " must be a number"
					}
					val = n
				}
			}
		default:
			s, _ := raw.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				switch f.Type {
				case "select":
					found := false
					for _, o := range f.Options {
						found = found || o == s
					}
					if !found {
						return nil, f.Label + ": invalid option"
					}
				case "date":
					if _, err := time.Parse("2006-01-02", s); err != nil {
						return nil, f.Label + " must be a date"
					}
				}
				val = s
			}
		}
		if val == nil {
			if f.Required {
				return nil, f.Label + " is required"
			}
			continue
		}
		_ = present
		out[f.Key] = val
	}
	return out, ""
}

// renderAnswers formats the answers as a Markdown list for the ticket description.
func renderAnswers(fields models.CatalogFields, data models.RequestData) string {
	var b strings.Builder
	for _, f := range fields {
		v, ok := data[f.Key]
		if !ok {
			continue
		}
		s := fmt.Sprint(v)
		if bv, isBool := v.(bool); isBool {
			s = "no"
			if bv {
				s = "yes"
			}
		}
		fmt.Fprintf(&b, "- **%s:** %s\n", f.Label, strings.ReplaceAll(s, "\n", " "))
	}
	return b.String()
}

// CreateServiceRequest POST /api/v1/customers/:customerId/service-requests
// {"catalog_item_id": 2, "title": "…", "description": "…", "answers": {…}}
// Orders a catalogue item: creates a service_request ticket routed to the
// item's group, with its checklist as fulfilment tasks and — when the item
// requires it — an approval round. Customer-portal users may order.
func CreateServiceRequest(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	customerID, ok := paramID(c, "customerId")
	if !ok {
		return
	}
	if requireCustomerAccess(customerID, userID, role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		CatalogItemID uint           `json:"catalog_item_id" binding:"required"`
		Title         string         `json:"title"`
		Description   string         `json:"description"`
		Answers       map[string]any `json:"answers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "catalog_item_id is required"})
		return
	}
	var item models.ServiceCatalogItem
	if database.DB.Where("id = ? AND is_active = ?", req.CatalogItemID, true).First(&item).Error != nil ||
		(item.CustomerID != nil && *item.CustomerID != customerID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "catalogue item not found"})
		return
	}
	data, msg := validateAnswers(item.Fields, req.Answers)
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg, "code": "invalid_answers"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = item.Name
	}
	desc := strings.TrimSpace(req.Description)
	if answers := renderAnswers(item.Fields, data); answers != "" {
		if desc != "" {
			desc += "\n\n"
		}
		desc += answers
	}

	cid := customerID
	itemID := item.ID
	ticket := models.Ticket{
		CustomerID: &cid, Title: title, Description: desc, Status: "new", CreatedByID: userID,
		GroupID: item.GroupID, CatalogItemID: &itemID, RequestData: data,
	}
	if errMsg := initNewTicket(&ticket, "service_request", "service_request", item.DefaultPriority, "", "", item.CategoryID); errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	if item.ApprovalsRequired > 0 && item.ApprovalGroupID != nil {
		services.StartApprovalRound(&ticket, *item.ApprovalGroupID, item.ApprovalsRequired)
	}
	if err := database.DB.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}
	database.DB.Create(&models.TicketHistory{TicketID: ticket.ID, UserID: userID, EventType: "created", Detail: "service catalogue: " + item.Name})
	if item.ChecklistTemplateID != nil {
		_ = applyChecklistTemplateToTicket(&ticket, *item.ChecklistTemplateID) // best effort: an inactive/empty template just adds no tasks
	}
	if ticket.ApprovalStatus == services.ApprovalPending {
		database.DB.Create(&models.TicketHistory{TicketID: ticket.ID, UserID: userID, EventType: "approval_requested"})
		services.NotifyApprovers(&ticket)
	}
	created, _ := preloadTicket(ticket.ID)
	attachTicketChecklist(&created)
	c.JSON(http.StatusCreated, created)
}
