package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

type changeCalendarEntry struct {
	ID           uint       `json:"id"`
	Title        string     `json:"title"`
	CustomerID   *uint      `json:"customer_id"`
	CustomerName string     `json:"customer_name"`
	ChangeType   string     `json:"change_type"`
	ChangeStatus string     `json:"change_status"`
	Risk         string     `json:"risk"`
	Priority     string     `json:"priority"`
	PlannedStart *time.Time `json:"planned_start"`
	PlannedEnd   *time.Time `json:"planned_end"`
}

// ListChanges GET /api/v1/changes?from=YYYY-MM-DD&to=YYYY-MM-DD&unscheduled=true
//
// The change calendar: accessible change requests whose planned window
// overlaps the range (default: this month ±1), plus — with unscheduled=true —
// open changes without a window, and the freeze windows in the range.
func ListChanges(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	to := from.AddDate(0, 3, 0)
	if v := c.Query("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return
		}
		from = t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return
		}
		to = t.AddDate(0, 0, 1)
	}

	q := database.DB.Model(&models.Ticket{}).Where("type = ? AND (is_spam = ? OR is_spam IS NULL)", "change_request", false)
	if ids, all := accessibleCustomerIDs(userID, role); !all {
		if len(ids) == 0 {
			q = q.Where("customer_id IS NULL")
		} else {
			q = q.Where("customer_id IS NULL OR customer_id IN ?", ids)
		}
	}
	window := "(planned_start < ? AND planned_end > ?)"
	if c.Query("unscheduled") == "true" {
		q = q.Where(window+" OR (planned_start IS NULL AND change_status NOT IN ?)", to, from, []string{"completed", "failed", "cancelled"})
	} else {
		q = q.Where(window, to, from)
	}
	var tickets []models.Ticket
	q.Preload("Customer").Order("planned_start asc, id asc").Limit(1000).Find(&tickets)
	changes := make([]changeCalendarEntry, 0, len(tickets))
	for _, t := range tickets {
		e := changeCalendarEntry{t.ID, t.Title, t.CustomerID, "", t.ChangeType, t.ChangeStatus, t.Risk, t.Priority, t.PlannedStart, t.PlannedEnd}
		if t.Customer != nil {
			e.CustomerName = t.Customer.Name
		}
		changes = append(changes, e)
	}

	var freezes []models.ChangeFreezeWindow
	database.DB.Where("start_at < ? AND end_at > ?", to, from).Order("start_at asc").Find(&freezes)
	visible := []models.ChangeFreezeWindow{}
	for _, f := range freezes {
		if canSeeCustomerScoped(f.CustomerID, userID, role, false) {
			visible = append(visible, f)
		}
	}
	c.JSON(http.StatusOK, gin.H{"from": from.Format("2006-01-02"), "to": to.AddDate(0, 0, -1).Format("2006-01-02"), "changes": changes, "freezes": visible})
}

// CheckChangeSchedule GET /customers/:customerId/tickets/:ticketId/change/check
// — freeze windows and conflicting changes (same CI, overlapping window) for
// the change's planned window.
func CheckChangeSchedule(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	if t.Type != "change_request" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not a change request"})
		return
	}
	check := services.CheckChangeSchedule(t)
	// Conflicting changes of customers the caller can't access are counted,
	// not listed.
	visible := check.Conflicts[:0]
	hidden := 0
	for _, cf := range check.Conflicts {
		if canSeeCustomerScoped(cf.CustomerID, userID, role, false) {
			visible = append(visible, cf)
		} else {
			hidden++
		}
	}
	check.Conflicts = visible
	c.JSON(http.StatusOK, gin.H{"freezes": check.Freezes, "conflicts": check.Conflicts, "hidden_conflicts": hidden})
}

// ── Freeze windows ─────────────────────────────────────────────────────────

// ListFreezeWindows GET /api/v1/change-freezes (agents; upcoming and current)
func ListFreezeWindows(c *gin.Context) {
	userID, role, ok := requireAgent(c)
	if !ok {
		return
	}
	q := database.DB.Order("start_at asc")
	if c.Query("all") != "true" {
		q = q.Where("end_at > ?", time.Now())
	}
	var rows []models.ChangeFreezeWindow
	q.Find(&rows)
	out := []models.ChangeFreezeWindow{}
	for _, f := range rows {
		if canSeeCustomerScoped(f.CustomerID, userID, role, false) {
			out = append(out, f)
		}
	}
	c.JSON(http.StatusOK, out)
}

type freezeRequest struct {
	Name       *string    `json:"name"`
	StartAt    *time.Time `json:"start_at"`
	EndAt      *time.Time `json:"end_at"`
	CustomerID *uint      `json:"customer_id"`
}

func (r *freezeRequest) apply(f *models.ChangeFreezeWindow) string {
	if r.Name != nil {
		f.Name = strings.TrimSpace(*r.Name)
	}
	if r.StartAt != nil {
		f.StartAt = *r.StartAt
	}
	if r.EndAt != nil {
		f.EndAt = *r.EndAt
	}
	if r.CustomerID != nil {
		if *r.CustomerID == 0 {
			f.CustomerID = nil
		} else {
			if database.DB.First(&models.Customer{}, *r.CustomerID).Error != nil {
				return "customer not found"
			}
			id := *r.CustomerID
			f.CustomerID = &id
		}
	}
	switch {
	case f.Name == "":
		return "name is required"
	case f.StartAt.IsZero() || f.EndAt.IsZero():
		return "start_at and end_at are required"
	case !f.EndAt.After(f.StartAt):
		return "the end must be after the start"
	}
	return ""
}

// AdminCreateFreezeWindow POST /api/v1/admin/change-freezes
func AdminCreateFreezeWindow(c *gin.Context) {
	var req freezeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request (timestamps must be RFC 3339)"})
		return
	}
	var f models.ChangeFreezeWindow
	if msg := req.apply(&f); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	database.DB.Create(&f)
	c.JSON(http.StatusCreated, f)
}

// AdminUpdateFreezeWindow PUT /api/v1/admin/change-freezes/:id
func AdminUpdateFreezeWindow(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var f models.ChangeFreezeWindow
	if database.DB.First(&f, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "freeze window not found"})
		return
	}
	var req freezeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request (timestamps must be RFC 3339)"})
		return
	}
	if msg := req.apply(&f); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	database.DB.Save(&f)
	c.JSON(http.StatusOK, f)
}

// AdminDeleteFreezeWindow DELETE /api/v1/admin/change-freezes/:id
func AdminDeleteFreezeWindow(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	database.DB.Delete(&models.ChangeFreezeWindow{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// saveChangeSettings stores the change part of PUT /admin/helpdesk/settings.
func saveChangeSettings(cabGroupID *uint, approvals, emergency *int) string {
	if cabGroupID != nil {
		val := ""
		if *cabGroupID != 0 {
			if database.DB.First(&models.UserGroup{}, *cabGroupID).Error != nil {
				return "CAB group not found"
			}
			val = strconv.FormatUint(uint64(*cabGroupID), 10)
		}
		saveSetting(services.SettingChangeCABGroup, val)
	}
	for _, s := range []struct {
		v   *int
		key string
	}{{approvals, services.SettingChangeApprovals}, {emergency, services.SettingChangeEmergencyApprovals}} {
		if s.v == nil {
			continue
		}
		if *s.v < 1 || *s.v > 20 {
			return "approvals required must be between 1 and 20"
		}
		saveSetting(s.key, strconv.Itoa(*s.v))
	}
	return ""
}
