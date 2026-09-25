package handlers

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// timerRoundMinutes is the unit a stopped timer's duration is rounded up to.
const timerRoundMinutes = 15

// timerSegment is one time entry produced by stopping a timer.
type timerSegment struct {
	Date      string // YYYY-MM-DD
	StartTime string // HH:MM
	EndTime   string // HH:MM; "00:00" when the segment runs to midnight
	Minutes   int
}

// timerSegments turns a timer that ran from start to end into time entries:
// the duration is rounded up to whole timerRoundMinutes (at least one unit),
// counted from start truncated to the minute, and the rounded span is split
// at every local midnight in loc so each entry stays within one day.
func timerSegments(start, end time.Time, loc *time.Location) []timerSegment {
	s := start.In(loc).Truncate(time.Minute)
	unit := time.Duration(timerRoundMinutes) * time.Minute
	elapsed := end.Sub(s)
	units := (elapsed + unit - 1) / unit
	if units < 1 {
		units = 1
	}
	e := s.Add(units * unit)

	var segs []timerSegment
	for cur := s; cur.Before(e); {
		midnight := time.Date(cur.Year(), cur.Month(), cur.Day()+1, 0, 0, 0, 0, loc)
		stop := e
		if midnight.Before(e) {
			stop = midnight
		}
		segs = append(segs, timerSegment{
			Date:      cur.Format("2006-01-02"),
			StartTime: cur.Format("15:04"),
			EndTime:   stop.Format("15:04"),
			Minutes:   int(stop.Sub(cur) / time.Minute),
		})
		cur = stop
	}
	return segs
}

// timerTarget is a project or customer the timer can be started on.
type timerTarget struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	TimeTrackingOnly bool   `json:"time_tracking_only"`
	// Projects only: the board project's own customer, if any.
	CustomerID   *uint  `json:"customer_id,omitempty"`
	CustomerName string `json:"customer_name,omitempty"`
}

type timerTargets struct {
	Projects  []timerTarget `json:"projects"`
	Customers []timerTarget `json:"customers"`
}

// loadTimerTargets returns the projects and customers userID may book time
// on, following the same rules as the Log Time tab: open board projects the
// user can view, time-tracking-only projects and customers they own or an
// admin created, and the customers they have access to.
func loadTimerTargets(userID uint, globalRole string) timerTargets {
	out := timerTargets{Projects: []timerTarget{}, Customers: []timerTarget{}}
	isAdmin := globalRole == "admin"

	var board []models.Project
	database.DB.Preload("Customer").
		Where("time_tracking_only = false AND is_closed = false AND is_archived = false").
		Order("name asc").Find(&board)
	for _, p := range board {
		if p.Customer != nil && p.Customer.IsHidden {
			continue
		}
		if services.RequireProjectRole(p.ID, userID, globalRole, "viewer") != nil {
			continue
		}
		t := timerTarget{ID: p.ID, Name: p.Name, CustomerID: p.CustomerID}
		if p.Customer != nil {
			t.CustomerName = p.Customer.Name
		}
		out.Projects = append(out.Projects, t)
	}

	var ttProjects []models.Project
	var ttCustomers []models.Customer
	if isAdmin {
		database.DB.Where("time_tracking_only = true").Order("name asc").Find(&ttProjects)
		database.DB.Where("time_tracking_only = true").Order("name asc").Find(&ttCustomers)
	} else {
		database.DB.Raw(`SELECT p.* FROM projects p JOIN users u ON u.id = p.created_by_id
			WHERE p.time_tracking_only = true AND p.deleted_at IS NULL
			  AND (p.created_by_id = ? OR u.global_role = 'admin') ORDER BY p.name ASC`, userID).Scan(&ttProjects)
		database.DB.Raw(`SELECT cu.* FROM customers cu JOIN users u ON u.id = cu.created_by_id
			WHERE cu.time_tracking_only = true
			  AND (cu.created_by_id = ? OR u.global_role = 'admin') ORDER BY cu.name ASC`, userID).Scan(&ttCustomers)
	}
	for _, p := range ttProjects {
		out.Projects = append(out.Projects, timerTarget{ID: p.ID, Name: p.Name, TimeTrackingOnly: true})
	}

	var customers []models.Customer
	if isAdmin {
		database.DB.Where("time_tracking_only = false AND is_hidden = false").Order("name asc").Find(&customers)
	} else {
		ids := make([]uint, 0)
		for id := range getAccessibleCustomerRoles(userID) {
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			database.DB.Where("id IN ? AND time_tracking_only = false AND is_hidden = false", ids).
				Order("name asc").Find(&customers)
		}
	}
	for _, cu := range customers {
		out.Customers = append(out.Customers, timerTarget{ID: cu.ID, Name: cu.Name})
	}
	for _, cu := range ttCustomers {
		out.Customers = append(out.Customers, timerTarget{ID: cu.ID, Name: cu.Name, TimeTrackingOnly: true})
	}
	sort.SliceStable(out.Customers, func(i, j int) bool {
		return strings.ToLower(out.Customers[i].Name) < strings.ToLower(out.Customers[j].Name)
	})
	return out
}

// timerLocation resolves the zone a timer runs in: the IANA name the client
// sent, else the user's own time zone setting, else UTC.
func timerLocation(requested string, userID uint) (*time.Location, string) {
	for _, name := range []string{requested, userTimeZone(userID)} {
		if name == "" {
			continue
		}
		if loc, err := time.LoadLocation(name); err == nil {
			return loc, name
		}
	}
	return time.UTC, "UTC"
}

func userTimeZone(userID uint) string {
	var tz string
	database.DB.Model(&models.User{}).Where("id = ?", userID).Pluck("timezone", &tz)
	return tz
}

// stopTimer books timer as time entries ending at end, and removes it.
func stopTimer(tx *gorm.DB, timer *models.TimeTimer, end time.Time) ([]models.TimeEntry, error) {
	loc, err := time.LoadLocation(timer.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	entries := []models.TimeEntry{}
	for _, seg := range timerSegments(timer.StartedAt, end, loc) {
		date, _ := time.Parse("2006-01-02", seg.Date)
		startTime, endTime := seg.StartTime, seg.EndTime
		entry := models.TimeEntry{
			UserID:      timer.UserID,
			CustomerID:  timer.CustomerID,
			ProjectID:   timer.ProjectID,
			Date:        date,
			Minutes:     seg.Minutes,
			Description: timer.Description,
			StartTime:   &startTime,
			EndTime:     &endTime,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := tx.Where("user_id = ?", timer.UserID).Delete(&models.TimeTimer{}).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// timerResponse is the GET /timer and POST /timer/start response.
type timerResponse struct {
	Running        bool               `json:"running"`
	Timer          *models.TimeTimer  `json:"timer,omitempty"`
	ElapsedMinutes int                `json:"elapsed_minutes,omitempty"`
	StoppedEntries []models.TimeEntry `json:"stopped_entries,omitempty"`
}

func loadTimer(userID uint) (*models.TimeTimer, error) {
	var timer models.TimeTimer
	err := database.DB.Preload("Customer").Preload("Project").Where("user_id = ?", userID).First(&timer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &timer, err
}

func preloadEntries(entries []models.TimeEntry) []models.TimeEntry {
	for i := range entries {
		database.DB.Preload("Customer").Preload("Project").First(&entries[i], entries[i].ID)
	}
	return entries
}

// GetTimer godoc
// @Summary      Show the running time-tracking timer
// @Description  Returns running=false when no timer is running.
// @Tags         time-tracking
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} timerResponse
// @Router       /timer [get]
func GetTimer(c *gin.Context) {
	timer, err := loadTimer(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if timer == nil {
		c.JSON(http.StatusOK, timerResponse{Running: false})
		return
	}
	c.JSON(http.StatusOK, timerResponse{
		Running:        true,
		Timer:          timer,
		ElapsedMinutes: int(time.Since(timer.StartedAt) / time.Minute),
	})
}

// GetTimerTargets godoc
// @Summary      List the projects and customers a timer can be started on
// @Tags         time-tracking
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} timerTargets
// @Router       /timer/targets [get]
func GetTimerTargets(c *gin.Context) {
	c.JSON(http.StatusOK, loadTimerTargets(middleware.GetUserID(c), middleware.GetGlobalRole(c)))
}

// StartTimer godoc
// @Summary      Start the time-tracking timer
// @Description  Needs project_id and/or customer_id (see GET /timer/targets). A board project brings its
// @Description  own customer. A timer that is already running is stopped first and booked; those entries
// @Description  are returned as stopped_entries. time_zone is an IANA zone name (default: the user's setting).
// @Tags         time-tracking
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body map[string]interface{} true "project_id, customer_id, description, time_zone"
// @Success      201 {object} timerResponse
// @Failure      400 {object} map[string]string
// @Router       /timer/start [post]
func StartTimer(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		ProjectID   *uint  `json:"project_id"`
		CustomerID  *uint  `json:"customer_id"`
		Description string `json:"description"`
		TimeZone    string `json:"time_zone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.ProjectID == nil && req.CustomerID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id or customer_id is required"})
		return
	}

	targets := loadTimerTargets(userID, middleware.GetGlobalRole(c))
	if req.ProjectID != nil {
		var project *timerTarget
		for i := range targets.Projects {
			if targets.Projects[i].ID == *req.ProjectID {
				project = &targets.Projects[i]
			}
		}
		if project == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project not available for time tracking"})
			return
		}
		if project.CustomerID != nil {
			if req.CustomerID != nil && *req.CustomerID != *project.CustomerID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "project belongs to another customer"})
				return
			}
			req.CustomerID = project.CustomerID
		}
	}
	if req.CustomerID != nil {
		found := false
		for _, cu := range targets.Customers {
			found = found || cu.ID == *req.CustomerID
		}
		// A board project's own customer is fine even without direct
		// customer access: the project access already covers it.
		if !found && (req.ProjectID == nil || !projectHasCustomer(targets, *req.ProjectID, *req.CustomerID)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer not available for time tracking"})
			return
		}
	}

	now := time.Now()
	_, zone := timerLocation(req.TimeZone, userID)
	if err := checkContractNotExpired(database.DB, req.CustomerID, req.ProjectID, nil, now); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var stopped []models.TimeEntry
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var running models.TimeTimer
		if tx.Where("user_id = ?", userID).First(&running).Error == nil {
			var err error
			if stopped, err = stopTimer(tx, &running, now); err != nil {
				return err
			}
		}
		return tx.Create(&models.TimeTimer{
			UserID:      userID,
			CustomerID:  req.CustomerID,
			ProjectID:   req.ProjectID,
			Description: strings.TrimSpace(req.Description),
			TimeZone:    zone,
			StartedAt:   now,
		}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start timer"})
		return
	}
	timer, _ := loadTimer(userID)
	c.JSON(http.StatusCreated, timerResponse{Running: true, Timer: timer, StoppedEntries: preloadEntries(stopped)})
}

func projectHasCustomer(targets timerTargets, projectID, customerID uint) bool {
	for _, p := range targets.Projects {
		if p.ID == projectID && p.CustomerID != nil && *p.CustomerID == customerID {
			return true
		}
	}
	return false
}

// StopTimer godoc
// @Summary      Stop the time-tracking timer and book the time
// @Description  The duration is rounded up to whole 15 minutes and booked as time entries, split at
// @Description  midnight. Optional "end" (RFC 3339) books a timer that was forgotten: it must lie between
// @Description  the start and now.
// @Tags         time-tracking
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body map[string]interface{} false "optional end (RFC 3339)"
// @Success      200 {array} models.TimeEntry
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /timer/stop [post]
func StopTimer(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		End *time.Time `json:"end"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end must be an RFC 3339 timestamp"})
			return
		}
	}
	timer, err := loadTimer(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if timer == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no timer running"})
		return
	}
	end := time.Now()
	if req.End != nil {
		if req.End.Before(timer.StartedAt) || req.End.After(end) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "end must lie between the timer's start and now"})
			return
		}
		end = *req.End
	}
	var entries []models.TimeEntry
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		entries, err = stopTimer(tx, timer, end)
		return err
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stop timer"})
		return
	}
	c.JSON(http.StatusOK, preloadEntries(entries))
}

// CancelTimer godoc
// @Summary      Discard the running timer without booking any time
// @Tags         time-tracking
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      204
// @Failure      404 {object} map[string]string
// @Router       /timer [delete]
func CancelTimer(c *gin.Context) {
	res := database.DB.Where("user_id = ?", middleware.GetUserID(c)).Delete(&models.TimeTimer{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no timer running"})
		return
	}
	c.Status(http.StatusNoContent)
}
