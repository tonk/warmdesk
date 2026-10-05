package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

var validCIStatuses = map[string]bool{"planned": true, "active": true, "maintenance": true, "retired": true}
var validCIEnvironments = map[string]bool{"": true, "production": true, "acceptance": true, "test": true, "development": true}
var validCIRelationTypes = map[string]bool{"depends_on": true, "runs_on": true, "part_of": true, "connected_to": true}

// The CMDB is agents-only: customer-portal users never see it. A CI with a
// customer follows that customer's access rules; an internal CI (no
// customer) is visible to every agent.
func canSeeCI(ci *models.ConfigurationItem, userID uint, role string) bool {
	return canSeeCustomerScoped(ci.CustomerID, userID, role, false)
}

func requireAgent(c *gin.Context) (uint, string, bool) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if requireNotCustomerRole(role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return 0, "", false
	}
	return userID, role, true
}

func loadVisibleCI(c *gin.Context, param string, userID uint, role string) (*models.ConfigurationItem, bool) {
	id, ok := paramID(c, param)
	if !ok {
		return nil, false
	}
	var ci models.ConfigurationItem
	if database.DB.Preload("Type").Preload("Customer").Preload("Owner").Preload("Group").First(&ci, id).Error != nil || !canSeeCI(&ci, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration item not found"})
		return nil, false
	}
	return &ci, true
}

// ── CI types ───────────────────────────────────────────────────────────────

// ListCITypes GET /api/v1/cmdb/types
func ListCITypes(c *gin.Context) {
	if _, _, ok := requireAgent(c); !ok {
		return
	}
	var types []models.CIType
	database.DB.Order("sort_order asc, name asc").Find(&types)
	if types == nil {
		types = []models.CIType{}
	}
	c.JSON(http.StatusOK, types)
}

// AdminCreateCIType POST /api/v1/admin/ci-types
func AdminCreateCIType(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Icon      string `json:"icon"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	t := models.CIType{Name: strings.TrimSpace(req.Name), Icon: req.Icon, SortOrder: req.SortOrder}
	if err := database.DB.Create(&t).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a type with that name already exists"})
		return
	}
	c.JSON(http.StatusCreated, t)
}

// AdminUpdateCIType PUT /api/v1/admin/ci-types/:id
func AdminUpdateCIType(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var t models.CIType
	if database.DB.First(&t, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "type not found"})
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Icon      *string `json:"icon"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u := map[string]any{}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}
		u["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Icon != nil {
		u["icon"] = *req.Icon
	}
	if req.SortOrder != nil {
		u["sort_order"] = *req.SortOrder
	}
	if err := database.DB.Model(&t).Updates(u).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a type with that name already exists"})
		return
	}
	database.DB.First(&t, t.ID)
	c.JSON(http.StatusOK, t)
}

// AdminDeleteCIType DELETE /api/v1/admin/ci-types/:id — refused while CIs use it.
func AdminDeleteCIType(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var n int64
	database.DB.Model(&models.ConfigurationItem{}).Where("type_id = ?", id).Count(&n)
	if n > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "configuration items still use this type", "code": "ci_type_in_use"})
		return
	}
	database.DB.Delete(&models.CIType{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ── Configuration items ────────────────────────────────────────────────────

// ListCIs GET /api/v1/cmdb/cis?q=&type_id=&customer_id=&status=&include_retired=
func ListCIs(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	q := database.DB.Model(&models.ConfigurationItem{})
	if ids, all := accessibleCustomerIDs(userID, role); !all {
		if len(ids) == 0 {
			q = q.Where("customer_id IS NULL")
		} else {
			q = q.Where("customer_id IS NULL OR customer_id IN ?", ids)
		}
	}
	if v := c.Query("type_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("type_id = ?", id)
		}
	}
	switch v := c.Query("customer_id"); {
	case v == "internal":
		q = q.Where("customer_id IS NULL")
	case v != "":
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("customer_id = ?", id)
		}
	}
	if v := c.Query("status"); validCIStatuses[v] {
		q = q.Where("status = ?", v)
	} else if c.Query("include_retired") != "true" {
		q = q.Where("status <> ?", "retired")
	}
	if term := strings.ToLower(strings.TrimSpace(c.Query("q"))); term != "" {
		like := "%" + term + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(attributes) LIKE ?", like, like, like)
	}
	var cis []models.ConfigurationItem
	q.Preload("Type").Preload("Customer").Preload("Owner").Preload("Group").Order("name asc").Limit(1000).Find(&cis)
	if cis == nil {
		cis = []models.ConfigurationItem{}
	}
	c.JSON(http.StatusOK, cis)
}

type ciRequest struct {
	Name        *string              `json:"name"`
	TypeID      *uint                `json:"type_id"`
	CustomerID  *uint                `json:"customer_id"`
	Status      *string              `json:"status"`
	Environment *string              `json:"environment"`
	Criticality *string              `json:"criticality"`
	OwnerID     *uint                `json:"owner_id"`
	GroupID     *uint                `json:"group_id"`
	Description *string              `json:"description"`
	Attributes  *models.CIAttributes `json:"attributes"`
}

func (r *ciRequest) updates(userID uint, role string) (map[string]any, int, string) {
	u := map[string]any{}
	if r.Name != nil {
		n := strings.TrimSpace(*r.Name)
		if n == "" {
			return nil, http.StatusBadRequest, "name is required"
		}
		u["name"] = n
	}
	if r.TypeID != nil {
		if database.DB.First(&models.CIType{}, *r.TypeID).Error != nil {
			return nil, http.StatusBadRequest, "type not found"
		}
		u["type_id"] = *r.TypeID
	}
	if r.CustomerID != nil {
		if *r.CustomerID != 0 && requireCustomerAccess(*r.CustomerID, userID, role) != nil {
			return nil, http.StatusForbidden, "no access to that customer"
		}
		u["customer_id"] = nullableID(r.CustomerID)
	}
	if r.Status != nil {
		if !validCIStatuses[*r.Status] {
			return nil, http.StatusBadRequest, "invalid status"
		}
		u["status"] = *r.Status
	}
	if r.Environment != nil {
		if !validCIEnvironments[*r.Environment] {
			return nil, http.StatusBadRequest, "invalid environment"
		}
		u["environment"] = *r.Environment
	}
	if r.Criticality != nil {
		if *r.Criticality != "" && !services.ValidImpactUrgency[*r.Criticality] {
			return nil, http.StatusBadRequest, "invalid criticality"
		}
		u["criticality"] = *r.Criticality
	}
	for _, ch := range []struct {
		id    *uint
		col   string
		model any
	}{{r.OwnerID, "owner_id", &models.User{}}, {r.GroupID, "group_id", &models.UserGroup{}}} {
		if ch.id == nil {
			continue
		}
		if *ch.id != 0 && database.DB.First(ch.model, *ch.id).Error != nil {
			return nil, http.StatusBadRequest, ch.col + " not found"
		}
		u[ch.col] = nullableID(ch.id)
	}
	if r.Description != nil {
		u["description"] = *r.Description
	}
	if r.Attributes != nil {
		clean := models.CIAttributes{}
		for k, v := range *r.Attributes {
			if k = strings.TrimSpace(k); k != "" {
				clean[k] = v
			}
		}
		u["attributes"] = clean
	}
	return u, 0, ""
}

// CreateCI POST /api/v1/cmdb/cis
func CreateCI(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	var req ciRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil || req.TypeID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and type_id are required"})
		return
	}
	u, status, msg := req.updates(userID, role)
	if msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	ci := models.ConfigurationItem{Name: u["name"].(string), TypeID: u["type_id"].(uint), Status: "active", Attributes: models.CIAttributes{}}
	if err := database.DB.Create(&ci).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create configuration item"})
		return
	}
	database.DB.Model(&ci).Updates(u)
	database.DB.Preload("Type").Preload("Customer").Preload("Owner").Preload("Group").First(&ci, ci.ID)
	c.JSON(http.StatusCreated, ci)
}

type ciRelationEntry struct {
	ID        uint                     `json:"id"`
	Type      string                   `json:"type"`
	Direction string                   `json:"direction"` // outgoing: this CI <type> other; incoming: other <type> this CI
	CI        models.ConfigurationItem `json:"ci"`
}

type ciTicketEntry struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	CustomerID *uint  `json:"customer_id"`
}

// GetCI GET /api/v1/cmdb/cis/:id — with relations (both directions) and linked tickets.
func GetCI(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	ci, ok := loadVisibleCI(c, "id", userID, role)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ci":        ci,
		"relations": ciRelations(ci.ID, userID, role),
		"tickets":   ciTickets(ci.ID, userID, role, false),
	})
}

func ciRelations(ciID, userID uint, role string) []ciRelationEntry {
	var rels []models.CIRelation
	database.DB.Where("source_id = ? OR target_id = ?", ciID, ciID).Order("id asc").Find(&rels)
	out := []ciRelationEntry{}
	for _, r := range rels {
		otherID, dir := r.TargetID, "outgoing"
		if r.TargetID == ciID {
			otherID, dir = r.SourceID, "incoming"
		}
		var other models.ConfigurationItem
		if database.DB.Preload("Type").First(&other, otherID).Error != nil || !canSeeCI(&other, userID, role) {
			continue
		}
		out = append(out, ciRelationEntry{ID: r.ID, Type: r.Type, Direction: dir, CI: other})
	}
	return out
}

func ciTickets(ciID, userID uint, role string, openOnly bool) []ciTicketEntry {
	q := database.DB.Model(&models.Ticket{}).
		Joins("JOIN ticket_cis ON ticket_cis.ticket_id = tickets.id").
		Where("ticket_cis.ci_id = ?", ciID)
	if openOnly {
		q = q.Where("tickets.status NOT IN ?", []string{"resolved", "pending_close", "closed"})
	}
	var tickets []models.Ticket
	q.Select("tickets.id", "tickets.title", "tickets.type", "tickets.status", "tickets.priority", "tickets.customer_id").
		Order("tickets.created_at desc").Limit(200).Find(&tickets)
	out := []ciTicketEntry{}
	for i := range tickets {
		if canSeeTicket(&tickets[i], userID, role) {
			t := tickets[i]
			out = append(out, ciTicketEntry{t.ID, t.Title, t.Type, t.Status, t.Priority, t.CustomerID})
		}
	}
	return out
}

// UpdateCI PUT /api/v1/cmdb/cis/:id
func UpdateCI(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	ci, ok := loadVisibleCI(c, "id", userID, role)
	if !ok {
		return
	}
	var req ciRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, status, msg := req.updates(userID, role)
	if msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	if len(u) > 0 {
		database.DB.Model(&models.ConfigurationItem{}).Where("id = ?", ci.ID).Updates(u)
	}
	var out models.ConfigurationItem
	database.DB.Preload("Type").Preload("Customer").Preload("Owner").Preload("Group").First(&out, ci.ID)
	c.JSON(http.StatusOK, out)
}

// DeleteCI DELETE /api/v1/cmdb/cis/:id — also removes its relations and ticket links.
func DeleteCI(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	ci, ok := loadVisibleCI(c, "id", userID, role)
	if !ok {
		return
	}
	database.DB.Where("source_id = ? OR target_id = ?", ci.ID, ci.ID).Delete(&models.CIRelation{})
	database.DB.Where("ci_id = ?", ci.ID).Delete(&models.TicketCI{})
	database.DB.Delete(&models.ConfigurationItem{}, ci.ID)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// CreateCIRelation POST /api/v1/cmdb/cis/:id/relations {"target_id": 7, "type": "depends_on"}
// read as "this CI depends on CI 7".
func CreateCIRelation(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	ci, ok := loadVisibleCI(c, "id", userID, role)
	if !ok {
		return
	}
	var req struct {
		TargetID uint   `json:"target_id"`
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TargetID == 0 || !validCIRelationTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_id and a valid type are required"})
		return
	}
	if req.TargetID == ci.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a configuration item can't relate to itself"})
		return
	}
	var target models.ConfigurationItem
	if database.DB.First(&target, req.TargetID).Error != nil || !canSeeCI(&target, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "target configuration item not found"})
		return
	}
	rel := models.CIRelation{SourceID: ci.ID, TargetID: target.ID, Type: req.Type}
	if err := database.DB.Create(&rel).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "that relation already exists"})
		return
	}
	database.DB.Preload("Type").First(&target, target.ID)
	c.JSON(http.StatusCreated, ciRelationEntry{ID: rel.ID, Type: rel.Type, Direction: "outgoing", CI: target})
}

// DeleteCIRelation DELETE /api/v1/cmdb/relations/:id
func DeleteCIRelation(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var rel models.CIRelation
	if database.DB.First(&rel, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "relation not found"})
		return
	}
	var src models.ConfigurationItem
	if database.DB.First(&src, rel.SourceID).Error != nil || !canSeeCI(&src, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "relation not found"})
		return
	}
	database.DB.Delete(&rel)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

type ciImpactEntry struct {
	CI          models.ConfigurationItem `json:"ci"`
	Depth       int                      `json:"depth"`
	Via         string                   `json:"via"` // relation type that makes it depend on its parent in the walk
	OpenTickets []ciTicketEntry          `json:"open_tickets"`
}

// impactedCIs walks the dependency graph upwards from ciID: every CI that
// depends on, runs on or is part of an impacted CI is impacted too
// (connected_to is not directional and is not followed). Breadth-first, up
// to maxDepth levels, each CI once.
func impactedCIs(ciID uint, maxDepth int) []struct {
	ID    uint
	Depth int
	Via   string
} {
	type hit = struct {
		ID    uint
		Depth int
		Via   string
	}
	seen := map[uint]bool{ciID: true}
	var out []hit
	frontier := []uint{ciID}
	for depth := 1; depth <= maxDepth && len(frontier) > 0; depth++ {
		var rels []models.CIRelation
		database.DB.Where("target_id IN ? AND type IN ?", frontier, []string{"depends_on", "runs_on", "part_of"}).Find(&rels)
		var next []uint
		for _, r := range rels {
			if seen[r.SourceID] {
				continue
			}
			seen[r.SourceID] = true
			out = append(out, hit{r.SourceID, depth, r.Type})
			next = append(next, r.SourceID)
		}
		frontier = next
	}
	return out
}

// GetCIImpact GET /api/v1/cmdb/cis/:id/impact — the CIs affected when this
// one fails or changes, with their open tickets. Impacted CIs the caller
// can't see are counted in "hidden" but not listed.
func GetCIImpact(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	ci, ok := loadVisibleCI(c, "id", userID, role)
	if !ok {
		return
	}
	out := []ciImpactEntry{}
	hidden := 0
	for _, h := range impactedCIs(ci.ID, 6) {
		var other models.ConfigurationItem
		if database.DB.Preload("Type").Preload("Customer").First(&other, h.ID).Error != nil {
			continue
		}
		if !canSeeCI(&other, userID, role) {
			hidden++
			continue
		}
		out = append(out, ciImpactEntry{CI: other, Depth: h.Depth, Via: h.Via, OpenTickets: ciTickets(other.ID, userID, role, true)})
	}
	c.JSON(http.StatusOK, gin.H{"impacted": out, "hidden": hidden, "open_tickets": ciTickets(ci.ID, userID, role, true)})
}

// ── Ticket ↔ CI links ──────────────────────────────────────────────────────

// ListTicketCIs GET /customers/:customerId/tickets/:ticketId/cis
func ListTicketCIs(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, false)
	if !ok {
		return
	}
	if role == "customer" {
		c.JSON(http.StatusOK, []models.ConfigurationItem{})
		return
	}
	var cis []models.ConfigurationItem
	database.DB.Joins("JOIN ticket_cis ON ticket_cis.ci_id = configuration_items.id").
		Where("ticket_cis.ticket_id = ?", t.ID).Preload("Type").Preload("Customer").
		Order("configuration_items.name asc").Find(&cis)
	out := []models.ConfigurationItem{}
	for i := range cis {
		if canSeeCI(&cis[i], userID, role) {
			out = append(out, cis[i])
		}
	}
	c.JSON(http.StatusOK, out)
}

// LinkTicketCI POST /customers/:customerId/tickets/:ticketId/cis {"ci_id": 5}
func LinkTicketCI(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	var req struct {
		CIID uint `json:"ci_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ci_id is required"})
		return
	}
	var ci models.ConfigurationItem
	if database.DB.Preload("Type").First(&ci, req.CIID).Error != nil || !canSeeCI(&ci, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration item not found"})
		return
	}
	var n int64
	database.DB.Model(&models.TicketCI{}).Where("ticket_id = ? AND ci_id = ?", t.ID, ci.ID).Count(&n)
	if n == 0 {
		database.DB.Create(&models.TicketCI{TicketID: t.ID, CIID: ci.ID, CreatedByID: userID})
		database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "ci_linked", Detail: ci.Name})
	}
	c.JSON(http.StatusCreated, ci)
}

// UnlinkTicketCI DELETE /customers/:customerId/tickets/:ticketId/cis/:ciId
func UnlinkTicketCI(c *gin.Context) {
	t, userID, _, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	ciID, ok := paramID(c, "ciId")
	if !ok {
		return
	}
	database.DB.Where("ticket_id = ? AND ci_id = ?", t.ID, ciID).Delete(&models.TicketCI{})
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "ci_unlinked"})
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// SeedCITypes creates a default set of CI types on first run.
func SeedCITypes() {
	var n int64
	database.DB.Model(&models.CIType{}).Count(&n)
	if n > 0 {
		return
	}
	for i, t := range []struct{ name, icon string }{
		{"Service", "🧩"}, {"Application", "📦"}, {"Server", "🖥"}, {"Database", "🗄"},
		{"Network device", "🌐"}, {"Workstation", "💻"}, {"Storage", "💾"},
	} {
		database.DB.Create(&models.CIType{Name: t.name, Icon: t.icon, SortOrder: i})
	}
}
