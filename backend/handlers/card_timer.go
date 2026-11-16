package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// Card timers: the time-tracking timer (handlers/timer.go) can run on a board
// card. It is started from the card's start/stop button or, automatically, by
// moving the card into a timed column (models.Column.Timed); moving it on to
// an untimed column stops and books it. Only the user who moves the card is
// affected — a timer belongs to one person, a card does not.

// truncateRunes cuts s to at most n runes.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// cardTimerDescription is the description a card timer resumes with: the one
// userID last booked on the card, else "KEY-n title".
func cardTimerDescription(userID uint, card *models.Card) string {
	var last models.TimeEntry
	if database.DB.Where("user_id = ? AND card_id = ? AND description <> ''", userID, card.ID).
		Order("date desc, id desc").First(&last).Error == nil {
		return last.Description
	}
	var prefix string
	database.DB.Model(&models.Project{}).Where("id = ?", card.ProjectID).Pluck("key_prefix", &prefix)
	if prefix != "" && card.CardNumber > 0 {
		return truncateRunes(fmt.Sprintf("%s-%d %s", prefix, card.CardNumber, card.Title), 255)
	}
	return truncateRunes(card.Title, 255)
}

// timeTrackingAvailable reports whether userID can use the timer at all: the
// server runs with time tracking, and the user has it enabled (or is admin).
func timeTrackingAvailable(userID uint, globalRole string) bool {
	if serverAppMode == "board" || serverAppMode == "ticketsystem" {
		return false
	}
	if globalRole == "admin" {
		return true
	}
	var enabled bool
	database.DB.Model(&models.User{}).Where("id = ?", userID).Pluck("time_tracking_enabled", &enabled)
	return enabled
}

// applyTimedColumn runs the timed-column rules after userID moved card from
// column fromID to toID. It never fails the move: a timer that can't start
// (no time tracking, project not bookable, expired contract) is just skipped.
func applyTimedColumn(userID uint, globalRole string, card *models.Card, fromID, toID uint) {
	if fromID == toID || !timeTrackingAvailable(userID, globalRole) {
		return
	}
	var to models.Column
	if err := database.DB.First(&to, toID).Error; err != nil {
		return
	}
	var running models.TimeTimer
	err := database.DB.Where("user_id = ?", userID).First(&running).Error
	hasRunning := err == nil
	onThisCard := hasRunning && running.CardID != nil && *running.CardID == card.ID

	if to.Timed {
		if onThisCard {
			return // timed → timed: keep timing
		}
		if _, err := startUserTimer(userID, globalRole, timerStart{CardID: &card.ID}); err != nil {
			var bad timerRequestError
			if !errors.As(err, &bad) {
				log.Printf("timed column: start timer for user %d on card %d: %v", userID, card.ID, err)
			}
		}
		return
	}
	if !onThisCard {
		return
	}
	var entries []models.TimeEntry
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var e error
		entries, e = stopTimer(tx, &running, time.Now())
		return e
	})
	if err != nil {
		log.Printf("timed column: stop timer for user %d on card %d: %v", userID, card.ID, err)
		return
	}
	notifyTimerChanged(userID, false, len(entries))
}

// cardTimeRow is booked time on a card, summed per user and description.
type cardTimeRow struct {
	UserID      uint   `json:"user_id"`
	UserName    string `json:"user_name"`
	Description string `json:"description"`
	Minutes     int    `json:"minutes"`
}

// cardTimeRunning is a timer currently running on the card.
type cardTimeRunning struct {
	UserID      uint      `json:"user_id"`
	UserName    string    `json:"user_name"`
	Description string    `json:"description"`
	StartedAt   time.Time `json:"started_at"`
}

type cardTimeResponse struct {
	TotalMinutes int               `json:"total_minutes"`
	Rows         []cardTimeRow     `json:"rows"`
	Running      []cardTimeRunning `json:"running"`
}

// GetCardTime godoc
// @Summary      Time booked on a card
// @Description  Time entries linked to the card (from card timers and timed columns), summed per user
// @Description  and description, plus the timers running on it right now.
// @Tags         cards
// @Produce      json
// @Security     BearerAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path int true "Card ID"
// @Success      200 {object} cardTimeResponse
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /projects/{projectSlug}/cards/{cardId}/time [get]
func GetCardTime(c *gin.Context) {
	userID := middleware.GetUserID(c)
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}
	project, err := services.GetProjectBySlug(c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var card models.Card
	if err := database.DB.Where("id = ? AND project_id = ?", cardID, project.ID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}

	out := cardTimeResponse{Rows: []cardTimeRow{}, Running: []cardTimeRunning{}}
	database.DB.Table("time_entries te").
		Select(`te.user_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS user_name,
			te.description, SUM(te.minutes) AS minutes`).
		Joins("JOIN users u ON u.id = te.user_id").
		Where("te.card_id = ?", card.ID).
		Group("te.user_id, u.display_name, u.username, te.description").
		Order("minutes desc").
		Scan(&out.Rows)
	for _, r := range out.Rows {
		out.TotalMinutes += r.Minutes
	}
	database.DB.Table("time_timers tt").
		Select(`tt.user_id, COALESCE(NULLIF(u.display_name, ''), u.username) AS user_name,
			tt.description, tt.started_at`).
		Joins("JOIN users u ON u.id = tt.user_id").
		Where("tt.card_id = ?", card.ID).
		Order("tt.started_at asc").
		Scan(&out.Running)
	c.JSON(http.StatusOK, out)
}
