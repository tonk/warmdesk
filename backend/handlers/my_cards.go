package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
)

// MyCardProject is the slim project summary embedded in a MyCardItem.
type MyCardProject struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	KeyPrefix    string `json:"key_prefix"`
	Color        string `json:"color"`
	CustomerID   *uint  `json:"customer_id"`
	CustomerName string `json:"customer_name"`
}

// MyCardItem is one open card assigned to the current user.
type MyCardItem struct {
	ID         uint          `json:"id"`
	CardNumber int           `json:"card_number"`
	Title      string        `json:"title"`
	Priority   string        `json:"priority"`
	DueDate    *time.Time    `json:"due_date"`
	ColumnName string        `json:"column_name"`
	UpdatedAt  time.Time     `json:"updated_at"`
	Project    MyCardProject `json:"project"`
}

// ListMyCards godoc
// @Summary      List all open cards assigned to the current user
// @Description  Covers both the legacy single assignee and the multi-assignee list. Only cards in open, non-deleted projects the user is (still) a member of are returned (admins: all projects).
// @Tags         cards
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} MyCardItem
// @Router       /my-cards [get]
func ListMyCards(c *gin.Context) {
	userID := middleware.GetUserID(c)
	globalRole := middleware.GetGlobalRole(c)

	// Projects the user may see: open, non-deleted board projects of visible
	// customers — the same set the dashboard lists.
	projQ := database.DB.Model(&models.Project{}).Preload("Customer").
		Where("is_closed = false AND time_tracking_only = false").
		Where("(customer_id NOT IN (?) OR customer_id IS NULL)",
			database.DB.Table("customers").Select("id").Where("is_hidden = true"))
	if globalRole != "admin" {
		memberIDs := database.DB.Model(&models.ProjectMember{}).Select("project_id").Where("user_id = ?", userID)
		projQ = projQ.Where("id IN (?)", memberIDs)
	}
	var projects []models.Project
	projQ.Find(&projects)
	if len(projects) == 0 {
		c.JSON(http.StatusOK, []MyCardItem{})
		return
	}
	projByID := make(map[uint]models.Project, len(projects))
	projectIDs := make([]uint, 0, len(projects))
	for _, p := range projects {
		projByID[p.ID] = p
		projectIDs = append(projectIDs, p.ID)
	}

	multi := database.DB.Model(&models.CardAssignee{}).Select("card_id").Where("user_id = ?", userID)
	var cards []models.Card
	database.DB.Preload("Column").
		Where("project_id IN ? AND closed = false", projectIDs).
		Where("assignee_id = ? OR id IN (?)", userID, multi).
		Find(&cards)

	result := make([]MyCardItem, 0, len(cards))
	for _, card := range cards {
		// Skip cards whose column was soft-deleted (Preload leaves it zero).
		if card.Column.ID == 0 {
			continue
		}
		p := projByID[card.ProjectID]
		mp := MyCardProject{
			ID: p.ID, Name: p.Name, Slug: p.Slug, KeyPrefix: p.KeyPrefix, Color: p.Color,
			CustomerID: p.CustomerID,
		}
		if p.Customer != nil {
			mp.CustomerName = p.Customer.Name
		}
		result = append(result, MyCardItem{
			ID:         card.ID,
			CardNumber: card.CardNumber,
			Title:      card.Title,
			Priority:   card.Priority,
			DueDate:    card.DueDate,
			ColumnName: card.Column.Name,
			UpdatedAt:  card.UpdatedAt,
			Project:    mp,
		})
	}

	// Due date first (soonest on top, no due date last), then project, then number.
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if (a.DueDate == nil) != (b.DueDate == nil) {
			return a.DueDate != nil
		}
		if a.DueDate != nil && !a.DueDate.Equal(*b.DueDate) {
			return a.DueDate.Before(*b.DueDate)
		}
		if a.Project.Name != b.Project.Name {
			return a.Project.Name < b.Project.Name
		}
		return a.CardNumber < b.CardNumber
	})

	c.JSON(http.StatusOK, result)
}
