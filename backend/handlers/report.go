package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/middleware"
	"github.com/tonk/coworker/models"
	"github.com/tonk/coworker/services"
)

type ReportCard struct {
	CardID           uint     `json:"card_id"`
	CardNumber       int      `json:"card_number"`
	CardRef          string   `json:"card_ref"`
	Title            string   `json:"title"`
	Assignees        []string `json:"assignees"`
	TimeSpentMinutes int      `json:"time_spent_minutes"`
	UpdatedAt        string   `json:"updated_at"`
	DueDate          *string  `json:"due_date"`
}

type ReportProject struct {
	ProjectID    uint         `json:"project_id"`
	ProjectName  string       `json:"project_name"`
	KeyPrefix    string       `json:"key_prefix"`
	Cards        []ReportCard `json:"cards"`
	TotalMinutes int          `json:"total_minutes"`
}

type TimeReportResponse struct {
	GeneratedAt  string          `json:"generated_at"`
	Period       string          `json:"period"`
	PeriodLabel  string          `json:"period_label"`
	Projects     []ReportProject `json:"projects"`
	TotalMinutes int             `json:"total_minutes"`
	CompanyName  string          `json:"company_name"`
	CompanyLogo  string          `json:"company_logo"`
}

// isoWeekStart returns the Monday that starts ISO week `week` of `year`.
func isoWeekStart(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := jan4.AddDate(0, 0, 1-weekday)
	_, jan4Week := jan4.ISOWeek()
	return monday.AddDate(0, 0, (week-jan4Week)*7)
}

// GetTimeReport godoc
// @Summary      Get time report data
// @Tags         reports
// @Produce      json
// @Security     BearerAuth
// @Param        period query string false "all|year|month|week"
// @Param        year query int false "Year filter"
// @Param        month query int false "Month filter"
// @Param        week query int false "Week filter"
// @Param        project query string false "Project slug filter"
// @Param        assignees query string false "Comma-separated user IDs"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} map[string]string
// @Router       /reports/time [get]
func GetTimeReport(c *gin.Context) {
	userID := middleware.GetUserID(c)
	globalRole := middleware.GetGlobalRole(c)

	if !userCanViewReports(userID, globalRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "reports are only available to project admins and system admins"})
		return
	}

	period := c.DefaultQuery("period", "all")
	projectSlug := c.Query("project")
	yearStr := c.Query("year")
	monthStr := c.Query("month")
	weekStr := c.Query("week")
	assigneesStr := c.Query("assignees")

	query := database.DB.Model(&models.Card{}).Where("time_spent_minutes > 0")

	if projectSlug != "" && projectSlug != "all" {
		project, err := services.GetProjectBySlug(projectSlug)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		if err := services.RequireProjectRole(project.ID, userID, globalRole, "viewer"); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		query = query.Where("project_id = ?", project.ID)
	} else if globalRole != "admin" {
		var memberProjectIDs []uint
		database.DB.Model(&models.ProjectMember{}).Where("user_id = ?", userID).Pluck("project_id", &memberProjectIDs)
		if len(memberProjectIDs) == 0 {
			settings := loadAllSettings()
			c.JSON(http.StatusOK, TimeReportResponse{
				GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04"),
				Period:      period,
				PeriodLabel: "All Time",
				Projects:    []ReportProject{},
				CompanyName: settings["company_name"],
				CompanyLogo: settings["company_logo"],
			})
			return
		}
		query = query.Where("project_id IN ?", memberProjectIDs)
	}

	now := time.Now().UTC()
	periodLabel := "All Time"
	switch period {
	case "year":
		year, _ := strconv.Atoi(yearStr)
		if year == 0 {
			year = now.Year()
		}
		periodLabel = fmt.Sprintf("Year %d", year)
		start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(1, 0, 0)
		query = query.Where("updated_at >= ? AND updated_at < ?", start, end)
	case "month":
		year, _ := strconv.Atoi(yearStr)
		month, _ := strconv.Atoi(monthStr)
		if year == 0 {
			year = now.Year()
		}
		if month == 0 {
			month = int(now.Month())
		}
		periodLabel = fmt.Sprintf("%s %d", time.Month(month).String(), year)
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		query = query.Where("updated_at >= ? AND updated_at < ?", start, end)
	case "week":
		year, _ := strconv.Atoi(yearStr)
		week, _ := strconv.Atoi(weekStr)
		if year == 0 {
			year = now.Year()
		}
		if week == 0 {
			_, week = now.ISOWeek()
		}
		start := isoWeekStart(year, week)
		end := start.AddDate(0, 0, 7)
		periodLabel = fmt.Sprintf("Week %d / %d", week, year)
		query = query.Where("updated_at >= ? AND updated_at < ?", start, end)
	}

	// Filter by assignees if specified
	if assigneesStr != "" {
		var assigneeIDs []uint
		for _, s := range strings.Split(assigneesStr, ",") {
			if id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil {
				assigneeIDs = append(assigneeIDs, uint(id))
			}
		}
		if len(assigneeIDs) > 0 {
			query = query.Where("id IN (SELECT card_id FROM card_assignees WHERE user_id IN ?)", assigneeIDs)
		}
	}

	var cards []models.Card
	query.Preload("Assignees").Find(&cards)

	projectIDSet := map[uint]bool{}
	for _, card := range cards {
		projectIDSet[card.ProjectID] = true
	}
	projectIDs := make([]uint, 0, len(projectIDSet))
	for id := range projectIDSet {
		projectIDs = append(projectIDs, id)
	}

	var projects []models.Project
	if len(projectIDs) > 0 {
		database.DB.Where("id IN ?", projectIDs).Find(&projects)
	}
	projectMap := map[uint]models.Project{}
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	reportProjectMap := map[uint]*ReportProject{}
	for _, card := range cards {
		proj := projectMap[card.ProjectID]
		if _, ok := reportProjectMap[card.ProjectID]; !ok {
			reportProjectMap[card.ProjectID] = &ReportProject{
				ProjectID:   proj.ID,
				ProjectName: proj.Name,
				KeyPrefix:   proj.KeyPrefix,
				Cards:       []ReportCard{},
			}
		}

		names := make([]string, 0, len(card.Assignees))
		for _, a := range card.Assignees {
			name := a.DisplayName
			if name == "" {
				name = a.Username
			}
			names = append(names, name)
		}

		cardRef := ""
		if proj.KeyPrefix != "" && card.CardNumber > 0 {
			cardRef = fmt.Sprintf("%s-%d", proj.KeyPrefix, card.CardNumber)
		}

		var dueDateStr *string
		if card.DueDate != nil {
			s := card.DueDate.Format("2006-01-02")
			dueDateStr = &s
		}

		rp := reportProjectMap[card.ProjectID]
		rp.Cards = append(rp.Cards, ReportCard{
			CardID:           card.ID,
			CardNumber:       card.CardNumber,
			CardRef:          cardRef,
			Title:            card.Title,
			Assignees:        names,
			TimeSpentMinutes: card.TimeSpentMinutes,
			UpdatedAt:        card.UpdatedAt.Format("2006-01-02"),
			DueDate:          dueDateStr,
		})
		rp.TotalMinutes += card.TimeSpentMinutes
	}

	reportProjects := make([]ReportProject, 0, len(reportProjectMap))
	totalMinutes := 0
	for _, rp := range reportProjectMap {
		totalMinutes += rp.TotalMinutes
		reportProjects = append(reportProjects, *rp)
	}
	sort.Slice(reportProjects, func(i, j int) bool {
		return reportProjects[i].ProjectName < reportProjects[j].ProjectName
	})

	settings := loadAllSettings()
	c.JSON(http.StatusOK, TimeReportResponse{
		GeneratedAt:  now.Format("2006-01-02 15:04"),
		Period:       period,
		PeriodLabel:  periodLabel,
		Projects:     reportProjects,
		TotalMinutes: totalMinutes,
		CompanyName:  settings["company_name"],
		CompanyLogo:  settings["company_logo"],
	})
}
