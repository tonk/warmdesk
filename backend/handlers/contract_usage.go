package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"gorm.io/gorm"
)

// ContractWeekUsage is how much of a contract's weekly hours have been booked
// in one ISO week (Monday through Sunday), counting the time entries of every
// user on that contract. Holiday entries don't count.
type ContractWeekUsage struct {
	ContractID    uint   `json:"contract_id"`
	WeekStart     string `json:"week_start"` // Monday, YYYY-MM-DD
	BookedMinutes int    `json:"booked_minutes"`
	LimitMinutes  int    `json:"limit_minutes"`
	Exceeded      bool   `json:"exceeded"`
}

// isoWeekMonday returns the Monday (00:00 UTC) of the ISO week holding d.
func isoWeekMonday(d time.Time) time.Time {
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

// contractWeekUsage returns the usage of con in the ISO week holding date,
// or nil when the contract has no hours per week set.
func contractWeekUsage(db *gorm.DB, con *models.Contract, date time.Time) *ContractWeekUsage {
	if con == nil || con.HoursPerWeek == nil {
		return nil
	}
	monday := isoWeekMonday(date)
	var booked int64
	db.Model(&models.TimeEntry{}).
		Where("contract_id = ? AND is_holiday = ? AND date >= ? AND date < ?",
			con.ID, false, monday, monday.AddDate(0, 0, 7)).
		Select("COALESCE(SUM(minutes), 0)").Scan(&booked)
	limit := int(math.Round(*con.HoursPerWeek * 60))
	return &ContractWeekUsage{
		ContractID:    con.ID,
		WeekStart:     monday.Format("2006-01-02"),
		BookedMinutes: int(booked),
		LimitMinutes:  limit,
		Exceeded:      int(booked) > limit,
	}
}

// timeEntryWithUsage is a saved time entry plus, when it is booked on a
// contract with hours per week, that contract's usage for the entry's week.
type timeEntryWithUsage struct {
	models.TimeEntry
	ContractUsage *ContractWeekUsage `json:"contract_usage,omitempty"`
}

func withContractUsage(entry models.TimeEntry) timeEntryWithUsage {
	return timeEntryWithUsage{
		TimeEntry:     entry,
		ContractUsage: contractWeekUsage(database.DB, entry.Contract, entry.Date),
	}
}

// GetContractUsage returns the weekly usage of the requested contracts for the
// ISO week holding ?date=YYYY-MM-DD. ?contract_ids is a comma-separated list.
// Contracts without hours per week, or of customers the caller can't access,
// are left out.
func GetContractUsage(c *gin.Context) {
	date, err := time.Parse("2006-01-02", c.Query("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
		return
	}
	var ids []uint
	for _, s := range strings.Split(c.Query("contract_ids"), ",") {
		if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil && id > 0 {
			ids = append(ids, uint(id))
		}
	}
	result := []ContractWeekUsage{}
	if len(ids) == 0 {
		c.JSON(http.StatusOK, result)
		return
	}
	var contracts []models.Contract
	database.DB.Where("id IN ? AND hours_per_week IS NOT NULL", ids).Order("id asc").Find(&contracts)
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	for i := range contracts {
		if requireCustomerAccess(contracts[i].CustomerID, userID, role) != nil {
			continue
		}
		if u := contractWeekUsage(database.DB, &contracts[i], date); u != nil {
			result = append(result, *u)
		}
	}
	c.JSON(http.StatusOK, result)
}
