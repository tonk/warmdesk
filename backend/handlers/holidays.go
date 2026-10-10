package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
)

// ── Handler ───────────────────────────────────────────────────────────────

type addHolidaysRequest struct {
	Year   int    `json:"year" binding:"required,min=2000,max=2100"`
	Locale string `json:"locale" binding:"required"`
	UserID *uint  `json:"user_id,omitempty"`
}

type holidayResult struct {
	Date  string `json:"date"`
	Name  string `json:"name"`
	Added bool   `json:"added"`
}

type addHolidaysResponse struct {
	Added    int             `json:"added"`
	Skipped  int             `json:"skipped"`
	Holidays []holidayResult `json:"holidays"`
}

func AddHolidays(c *gin.Context) {
	userID := middleware.GetUserID(c)
	globalRole := middleware.GetGlobalRole(c)

	var req addHolidaysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	holidays, ok := services.PublicHolidays(req.Locale, req.Year)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported locale"})
		return
	}

	targetUserID := userID
	if req.UserID != nil && globalRole == "admin" {
		targetUserID = *req.UserID
	}

	var results []holidayResult
	added, skipped := 0, 0

	for _, h := range holidays {
		var count int64
		database.DB.Model(&models.TimeEntry{}).
			Where("user_id = ? AND date = ?", targetUserID, h.Date).
			Count(&count)

		if count > 0 {
			results = append(results, holidayResult{Date: h.Date.Format("2006-01-02"), Name: h.Name, Added: false})
			skipped++
			continue
		}

		entry := models.TimeEntry{
			UserID:      targetUserID,
			Date:        h.Date,
			Minutes:     0,
			Description: h.Name,
			IsHoliday:   true,
		}
		if err := database.DB.Create(&entry).Error; err == nil {
			results = append(results, holidayResult{Date: h.Date.Format("2006-01-02"), Name: h.Name, Added: true})
			added++
		}
	}

	c.JSON(http.StatusOK, addHolidaysResponse{Added: added, Skipped: skipped, Holidays: results})
}
