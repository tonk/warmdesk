package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// ── Ticket categories ──────────────────────────────────────────────────────

// ListTicketCategories GET /api/v1/ticket-categories (active only, for agents)
func ListTicketCategories(c *gin.Context) {
	var cats []models.TicketCategory
	database.DB.Where("is_active = ?", true).Order("sort_order asc, name asc").Find(&cats)
	if cats == nil {
		cats = []models.TicketCategory{}
	}
	c.JSON(http.StatusOK, cats)
}

// AdminListTicketCategories GET /api/v1/admin/ticket-categories
func AdminListTicketCategories(c *gin.Context) {
	var cats []models.TicketCategory
	database.DB.Order("sort_order asc, name asc").Find(&cats)
	if cats == nil {
		cats = []models.TicketCategory{}
	}
	c.JSON(http.StatusOK, cats)
}

type categoryRequest struct {
	Name      *string `json:"name"`
	ParentID  *uint   `json:"parent_id"`
	IsActive  *bool   `json:"is_active"`
	SortOrder *int    `json:"sort_order"`
}

func (r *categoryRequest) updates(selfID uint) (map[string]any, string) {
	u := map[string]any{}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return nil, "name is required"
		}
		u["name"] = name
	}
	if r.ParentID != nil {
		if *r.ParentID != 0 {
			if *r.ParentID == selfID {
				return nil, "a category cannot be its own parent"
			}
			var parent models.TicketCategory
			if database.DB.First(&parent, *r.ParentID).Error != nil {
				return nil, "parent category not found"
			}
			// Two levels only: a subcategory can't have children of its own.
			if parent.ParentID != nil {
				return nil, "subcategories cannot have subcategories"
			}
			if selfID != 0 {
				var children int64
				database.DB.Model(&models.TicketCategory{}).Where("parent_id = ?", selfID).Count(&children)
				if children > 0 {
					return nil, "a category with subcategories cannot become a subcategory"
				}
			}
		}
		u["parent_id"] = nullableID(r.ParentID)
	}
	if r.IsActive != nil {
		u["is_active"] = *r.IsActive
	}
	if r.SortOrder != nil {
		u["sort_order"] = *r.SortOrder
	}
	return u, ""
}

// AdminCreateTicketCategory POST /api/v1/admin/ticket-categories
func AdminCreateTicketCategory(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	u, errMsg := req.updates(0)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	cat := models.TicketCategory{Name: u["name"].(string), IsActive: true}
	if err := database.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create category"})
		return
	}
	database.DB.Model(&cat).Updates(u)
	database.DB.First(&cat, cat.ID)
	c.JSON(http.StatusCreated, cat)
}

// AdminUpdateTicketCategory PUT /api/v1/admin/ticket-categories/:id
func AdminUpdateTicketCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var cat models.TicketCategory
	if database.DB.First(&cat, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, errMsg := req.updates(cat.ID)
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	if len(u) > 0 {
		database.DB.Model(&cat).Updates(u)
	}
	database.DB.First(&cat, cat.ID)
	c.JSON(http.StatusOK, cat)
}

// AdminDeleteTicketCategory DELETE /api/v1/admin/ticket-categories/:id
// Tickets and SLA policies referring to the category (or its subcategories)
// lose the reference; subcategories are deleted with their parent.
func AdminDeleteTicketCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ids := []uint{uint(id)}
	var childIDs []uint
	database.DB.Model(&models.TicketCategory{}).Where("parent_id = ?", id).Pluck("id", &childIDs)
	ids = append(ids, childIDs...)
	database.DB.Model(&models.Ticket{}).Where("category_id IN ?", ids).Update("category_id", nil)
	database.DB.Model(&models.SlaPolicy{}).Where("category_id IN ?", ids).Update("category_id", nil)
	database.DB.Where("id IN ?", ids).Delete(&models.TicketCategory{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ── Resolution codes ───────────────────────────────────────────────────────

// ListTicketResolutionCodes GET /api/v1/ticket-resolution-codes (active only)
func ListTicketResolutionCodes(c *gin.Context) {
	var codes []models.TicketResolutionCode
	database.DB.Where("is_active = ?", true).Order("sort_order asc, name asc").Find(&codes)
	if codes == nil {
		codes = []models.TicketResolutionCode{}
	}
	c.JSON(http.StatusOK, codes)
}

// AdminListTicketResolutionCodes GET /api/v1/admin/ticket-resolution-codes
func AdminListTicketResolutionCodes(c *gin.Context) {
	var codes []models.TicketResolutionCode
	database.DB.Order("sort_order asc, name asc").Find(&codes)
	if codes == nil {
		codes = []models.TicketResolutionCode{}
	}
	c.JSON(http.StatusOK, codes)
}

type resolutionCodeRequest struct {
	Name      *string `json:"name"`
	IsActive  *bool   `json:"is_active"`
	SortOrder *int    `json:"sort_order"`
}

func (r *resolutionCodeRequest) updates() (map[string]any, string) {
	u := map[string]any{}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return nil, "name is required"
		}
		u["name"] = name
	}
	if r.IsActive != nil {
		u["is_active"] = *r.IsActive
	}
	if r.SortOrder != nil {
		u["sort_order"] = *r.SortOrder
	}
	return u, ""
}

// AdminCreateTicketResolutionCode POST /api/v1/admin/ticket-resolution-codes
func AdminCreateTicketResolutionCode(c *gin.Context) {
	var req resolutionCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	u, errMsg := req.updates()
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	rc := models.TicketResolutionCode{Name: u["name"].(string), IsActive: true}
	if err := database.DB.Create(&rc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create resolution code"})
		return
	}
	database.DB.Model(&rc).Updates(u)
	database.DB.First(&rc, rc.ID)
	c.JSON(http.StatusCreated, rc)
}

// AdminUpdateTicketResolutionCode PUT /api/v1/admin/ticket-resolution-codes/:id
func AdminUpdateTicketResolutionCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var rc models.TicketResolutionCode
	if database.DB.First(&rc, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "resolution code not found"})
		return
	}
	var req resolutionCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, errMsg := req.updates()
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}
	if len(u) > 0 {
		database.DB.Model(&rc).Updates(u)
	}
	database.DB.First(&rc, rc.ID)
	c.JSON(http.StatusOK, rc)
}

// AdminDeleteTicketResolutionCode DELETE /api/v1/admin/ticket-resolution-codes/:id
// Deleting a code in use would erase history from resolved tickets, so codes
// that are referenced can only be deactivated.
func AdminDeleteTicketResolutionCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var used int64
	database.DB.Model(&models.Ticket{}).Where("resolution_code_id = ?", id).Count(&used)
	if used > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "resolution code is in use; deactivate it instead", "code": "resolution_code_in_use"})
		return
	}
	database.DB.Delete(&models.TicketResolutionCode{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ── Business calendars ─────────────────────────────────────────────────────

// AdminListBusinessCalendars GET /api/v1/admin/business-calendars
func AdminListBusinessCalendars(c *gin.Context) {
	var cals []models.BusinessCalendar
	database.DB.Preload("Holidays", func(db *gorm.DB) *gorm.DB { return db.Order("date asc") }).Order("name asc").Find(&cals)
	if cals == nil {
		cals = []models.BusinessCalendar{}
	}
	c.JSON(http.StatusOK, cals)
}

type calendarRequest struct {
	Name      *string `json:"name"`
	Timezone  *string `json:"timezone"`
	WorkDays  *string `json:"work_days"`
	StartTime *string `json:"start_time"`
	EndTime   *string `json:"end_time"`
	Holidays  *[]struct {
		Date string `json:"date"`
		Name string `json:"name"`
	} `json:"holidays"`
}

// saveCalendar validates and stores a calendar (create when cal.ID == 0),
// replacing its holiday list when the request includes one.
func saveCalendar(c *gin.Context, cal *models.BusinessCalendar, req *calendarRequest) bool {
	if req.Name != nil {
		cal.Name = strings.TrimSpace(*req.Name)
	}
	if req.Timezone != nil {
		cal.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.WorkDays != nil {
		cal.WorkDays = strings.TrimSpace(*req.WorkDays)
	}
	if req.StartTime != nil {
		cal.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		cal.EndTime = *req.EndTime
	}
	if cal.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return false
	}
	if cal.Timezone == "" {
		cal.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(cal.Timezone); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown timezone"})
		return false
	}
	if !services.ValidCalendar(cal) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "calendar needs at least one working day and an end time after the start time", "code": "invalid_calendar"})
		return false
	}
	var holidays []models.BusinessCalendarHoliday
	if req.Holidays != nil {
		seen := map[string]bool{}
		for _, h := range *req.Holidays {
			if _, err := time.Parse("2006-01-02", h.Date); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid holiday date " + h.Date})
				return false
			}
			if seen[h.Date] {
				continue
			}
			seen[h.Date] = true
			holidays = append(holidays, models.BusinessCalendarHoliday{Date: h.Date, Name: strings.TrimSpace(h.Name)})
		}
	}
	cal.Holidays = nil
	if err := database.DB.Omit("Holidays").Save(cal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save calendar"})
		return false
	}
	if req.Holidays != nil {
		database.DB.Where("calendar_id = ?", cal.ID).Delete(&models.BusinessCalendarHoliday{})
		for i := range holidays {
			holidays[i].CalendarID = cal.ID
			database.DB.Create(&holidays[i])
		}
	}
	database.DB.Preload("Holidays", func(db *gorm.DB) *gorm.DB { return db.Order("date asc") }).First(cal, cal.ID)
	return true
}

// AdminCreateBusinessCalendar POST /api/v1/admin/business-calendars
func AdminCreateBusinessCalendar(c *gin.Context) {
	var req calendarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	cal := models.BusinessCalendar{Timezone: "UTC", WorkDays: "1,2,3,4,5", StartTime: "09:00", EndTime: "17:00"}
	if saveCalendar(c, &cal, &req) {
		c.JSON(http.StatusCreated, cal)
	}
}

// AdminUpdateBusinessCalendar PUT /api/v1/admin/business-calendars/:id
func AdminUpdateBusinessCalendar(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var cal models.BusinessCalendar
	if database.DB.First(&cal, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "calendar not found"})
		return
	}
	var req calendarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if saveCalendar(c, &cal, &req) {
		c.JSON(http.StatusOK, cal)
	}
}

// AdminDeleteBusinessCalendar DELETE /api/v1/admin/business-calendars/:id
// SLA policies using the calendar fall back to 24×7.
func AdminDeleteBusinessCalendar(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	database.DB.Model(&models.SlaPolicy{}).Where("calendar_id = ?", id).Update("calendar_id", nil)
	database.DB.Where("calendar_id = ?", id).Delete(&models.BusinessCalendarHoliday{})
	database.DB.Delete(&models.BusinessCalendar{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// AdminImportCalendarHolidays POST /api/v1/admin/business-calendars/:id/import-holidays
// Body: {"year": 2027, "locale": "nl"} — adds that country's public holidays
// (the same tables as the time-tracking holiday import), skipping dates
// already present.
func AdminImportCalendarHolidays(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var cal models.BusinessCalendar
	if database.DB.First(&cal, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "calendar not found"})
		return
	}
	var req struct {
		Year   int    `json:"year" binding:"required,min=2000,max=2100"`
		Locale string `json:"locale" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "year and locale are required"})
		return
	}
	fn, ok := holidaysByLocale[req.Locale]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported locale"})
		return
	}
	added := 0
	for _, h := range fn(req.Year, easterSunday(req.Year)) {
		date := h.Date.Format("2006-01-02")
		var count int64
		database.DB.Model(&models.BusinessCalendarHoliday{}).Where("calendar_id = ? AND date = ?", cal.ID, date).Count(&count)
		if count > 0 {
			continue
		}
		database.DB.Create(&models.BusinessCalendarHoliday{CalendarID: cal.ID, Date: date, Name: h.Name})
		added++
	}
	database.DB.Preload("Holidays", func(db *gorm.DB) *gorm.DB { return db.Order("date asc") }).First(&cal, cal.ID)
	c.JSON(http.StatusOK, gin.H{"added": added, "calendar": cal})
}

// ── Helpdesk settings ──────────────────────────────────────────────────────

// GetHelpdeskSettings GET /api/v1/helpdesk/settings — readable by every
// helpdesk user so the UI can preview derived priorities and know whether a
// resolution code is required.
func GetHelpdeskSettings(c *gin.Context) {
	c.JSON(http.StatusOK, services.LoadHelpdeskSettings())
}

// AdminUpdateHelpdeskSettings PUT /api/v1/admin/helpdesk/settings
func AdminUpdateHelpdeskSettings(c *gin.Context) {
	var req struct {
		PriorityMatrix        map[string]string `json:"priority_matrix"`
		ResolvedAutoCloseDays *int              `json:"resolved_autoclose_days"`
		RequireResolutionCode *bool             `json:"require_resolution_code"`
		MajorIncidentGroupID  *uint             `json:"major_incident_group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.PriorityMatrix != nil {
		clean := map[string]string{}
		for k, v := range req.PriorityMatrix {
			if _, ok := services.DefaultPriorityMatrix[k]; !ok || !services.ValidTicketPriorities[v] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid priority matrix entry " + k})
				return
			}
			clean[k] = v
		}
		b, _ := json.Marshal(clean)
		saveSetting(services.SettingHelpdeskPriorityMatrix, string(b))
	}
	if req.ResolvedAutoCloseDays != nil {
		if *req.ResolvedAutoCloseDays < 0 || *req.ResolvedAutoCloseDays > 365 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resolved_autoclose_days must be between 0 and 365"})
			return
		}
		saveSetting(services.SettingHelpdeskResolvedAutoClose, strconv.Itoa(*req.ResolvedAutoCloseDays))
	}
	if req.RequireResolutionCode != nil {
		saveSetting(services.SettingHelpdeskRequireResolution, strconv.FormatBool(*req.RequireResolutionCode))
	}
	if req.MajorIncidentGroupID != nil {
		val := ""
		if *req.MajorIncidentGroupID != 0 {
			if database.DB.First(&models.UserGroup{}, *req.MajorIncidentGroupID).Error != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "group not found"})
				return
			}
			val = strconv.FormatUint(uint64(*req.MajorIncidentGroupID), 10)
		}
		saveSetting(services.SettingHelpdeskMajorIncidentGroup, val)
	}
	c.JSON(http.StatusOK, services.LoadHelpdeskSettings())
}

// ── Problem management ─────────────────────────────────────────────────────

type problemEntry struct {
	ID              uint      `json:"id"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	Priority        string    `json:"priority"`
	ProblemStatus   string    `json:"problem_status"`
	IsKnownError    bool      `json:"is_known_error"`
	RootCause       string    `json:"root_cause"`
	Workaround      string    `json:"workaround"`
	CustomerID      *uint     `json:"customer_id"`
	CustomerName    string    `json:"customer_name"`
	LinkedIncidents int64     `json:"linked_incidents"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// accessibleCustomerIDs returns the customer IDs a user may see tickets of;
// all=true for admins (no restriction).
func accessibleCustomerIDs(userID uint, role string) (ids []uint, all bool) {
	if role == "admin" {
		return nil, true
	}
	for id := range getAccessibleCustomerRoles(userID) {
		ids = append(ids, id)
	}
	return ids, false
}

// ListProblems GET /api/v1/problems?known_error=true&q=…&include_resolved=true
//
// Problem records across every customer the agent can access (plus inbox
// problems) — the known error database when known_error=true.
func ListProblems(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if err := requireNotCustomerRole(role); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	q := database.DB.Model(&models.Ticket{}).Where("type = ? AND (is_spam = ? OR is_spam IS NULL)", "problem", false)
	if ids, all := accessibleCustomerIDs(userID, role); !all {
		if len(ids) == 0 {
			q = q.Where("customer_id IS NULL")
		} else {
			q = q.Where("customer_id IS NULL OR customer_id IN ?", ids)
		}
	}
	if c.Query("known_error") == "true" {
		q = q.Where("is_known_error = ?", true)
	}
	if c.Query("include_resolved") != "true" {
		q = q.Where("status <> ?", "closed")
	}
	if term := trimmedOrEmpty(c.Query("q")); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(root_cause) LIKE ? OR LOWER(workaround) LIKE ? OR LOWER(description) LIKE ?", like, like, like, like)
	}
	var tickets []models.Ticket
	q.Preload("Customer").Order("updated_at desc").Limit(500).Find(&tickets)

	out := make([]problemEntry, 0, len(tickets))
	for _, t := range tickets {
		e := problemEntry{
			ID: t.ID, Title: t.Title, Status: t.Status, Priority: t.Priority,
			ProblemStatus: t.ProblemStatus, IsKnownError: t.IsKnownError,
			RootCause: t.RootCause, Workaround: t.Workaround, CustomerID: t.CustomerID,
			CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		}
		if t.Customer != nil {
			e.CustomerName = t.Customer.Name
		}
		database.DB.Model(&models.TicketLink{}).Where("target_ticket_id = ? AND link_type = ?", t.ID, "caused_by").Count(&e.LinkedIncidents)
		out = append(out, e)
	}
	c.JSON(http.StatusOK, out)
}

// ResolveLinkedIncidents POST /api/v1/customers/:customerId/tickets/:ticketId/resolve-linked
//
// Body: {"resolution_code_id": 3, "message": "…"}. Resolves every open
// incident linked to this problem as "caused_by" that the caller can access,
// optionally posting message as a public reply (emailed to the requester).
func ResolveLinkedIncidents(c *gin.Context) {
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
	var problem models.Ticket
	if err := database.DB.Where("id = ? AND customer_id = ? AND type = ?", ticketID, customerID, "problem").First(&problem).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}
	var req struct {
		ResolutionCodeID *uint  `json:"resolution_code_id"`
		Message          string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	settings := services.LoadHelpdeskSettings()
	if req.ResolutionCodeID != nil && *req.ResolutionCodeID != 0 {
		if database.DB.First(&models.TicketResolutionCode{}, *req.ResolutionCodeID).Error != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "resolution code not found"})
			return
		}
	} else if settings.RequireResolutionCode {
		c.JSON(http.StatusBadRequest, gin.H{"error": errResolutionCodeRequired, "code": "resolution_code_required"})
		return
	}

	var incidentIDs []uint
	database.DB.Model(&models.TicketLink{}).Where("target_ticket_id = ? AND link_type = ?", problem.ID, "caused_by").Pluck("source_ticket_id", &incidentIDs)
	resolved, skipped := 0, 0
	for _, iid := range incidentIDs {
		var inc models.Ticket
		if database.DB.First(&inc, iid).Error != nil {
			continue
		}
		if services.IsResolvedStatus(inc.Status) {
			continue
		}
		if !canSeeTicket(&inc, userID, role) || services.TicketChecklistBlocksClose(inc.ID) {
			skipped++
			continue
		}
		upd := ticketUpdateRequest{Status: strPtr("resolved")}
		if req.ResolutionCodeID != nil && *req.ResolutionCodeID != 0 {
			upd.ResolutionCode = req.ResolutionCodeID
		}
		notes := "Resolved via problem #" + strconv.FormatUint(uint64(problem.ID), 10)
		upd.ResolutionNotes = &notes
		if applyTicketUpdate(&inc, &upd, userID, false) != nil {
			skipped++
			continue
		}
		if msg := strings.TrimSpace(req.Message); msg != "" {
			m := models.TicketMessage{TicketID: inc.ID, UserID: userID, Body: msg}
			if database.DB.Create(&m).Error == nil {
				onTicketMessage(&inc, userID, role, false)
				sendEmailReply(&inc, msg)
				if inc.FromEmail != nil && *inc.FromEmail != "" {
					database.DB.Model(&m).Update("email_sent", true)
				}
			}
		}
		resolved++
	}
	database.DB.Create(&models.TicketHistory{TicketID: problem.ID, UserID: userID, EventType: "linked_incidents_resolved", Detail: strconv.Itoa(resolved)})
	c.JSON(http.StatusOK, gin.H{"resolved": resolved, "skipped": skipped})
}

func strPtr(s string) *string { return &s }
