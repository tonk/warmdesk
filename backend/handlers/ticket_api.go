package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	appws "github.com/tonk/warmdesk/ws"
	"gorm.io/gorm"
)

// TicketAdd godoc
// @Summary      Create a card via API key (CI/CD integration)
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        body body map[string]interface{} true "Card details (title required)"
// @Success      201 {object} models.Card
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards [post]
func TicketAdd(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "member"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		ColumnID    uint   `json:"column_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var col models.Column
	if err := database.DB.Where("id = ? AND project_id = ?", req.ColumnID, project.ID).First(&col).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "column not found in project"})
		return
	}

	var maxPos struct{ Pos float64 }
	database.DB.Model(&models.Card{}).Select("COALESCE(MAX(position), 0) as pos").Where("column_id = ?", col.ID).Scan(&maxPos)

	// Atomically increment the project's card counter
	database.DB.Model(&models.Project{}).Where("id = ?", project.ID).
		UpdateColumn("card_counter", gorm.Expr("card_counter + 1"))
	var updatedProject models.Project
	database.DB.Select("card_counter").First(&updatedProject, project.ID)

	card := models.Card{
		ColumnID:    col.ID,
		ProjectID:   project.ID,
		Title:       req.Title,
		Description: req.Description,
		Position:    maxPos.Pos + 1000,
		CreatedByID: userID,
		CardNumber:  updatedProject.CardCounter,
	}
	if err := database.DB.Create(&card).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	database.DB.Create(&models.CardHistory{CardID: card.ID, UserID: userID, EventType: "created"})
	database.DB.Preload("CreatedBy").Preload("Assignee").Preload("Labels").Preload("Tags").First(&card, card.ID)

	appws.BroadcastToProject(project.ID, appws.Message{Type: appws.TypeBoardCardCreated, Payload: card})
	c.JSON(http.StatusCreated, card)
}

// TicketComment adds a comment to a card via API key authentication.
// TicketComment godoc
// @Summary      Add a comment to a card via API key
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path int true "Card ID"
// @Param        body body map[string]string true "Comment body"
// @Success      201 {object} models.CardComment
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId}/comments [post]
func TicketComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}

	var card models.Card
	if err := database.DB.Where("id = ? AND project_id = ?", cardID, project.ID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}

	var req struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	comment := models.CardComment{CardID: card.ID, UserID: userID, Body: req.Body}
	if err := database.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	database.DB.Create(&models.CardHistory{CardID: card.ID, UserID: userID, EventType: "comment_added"})
	database.DB.Preload("User").First(&comment, comment.ID)

	appws.BroadcastToProject(project.ID, appws.Message{Type: appws.TypeBoardCommentCreated, Payload: comment})
	c.JSON(http.StatusCreated, comment)
}

// TicketMove moves a card to another column via API key authentication.
// PATCH /api/v1/ticket/:projectSlug/cards/:cardId/move
// TicketMove godoc
// @Summary      Move a card to a different column via API key
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path int true "Card ID"
// @Param        body body map[string]interface{} true "column_id and optional position"
// @Success      200 {object} models.Card
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId}/move [patch]
func TicketMove(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "member"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}

	var card models.Card
	if err := database.DB.Where("id = ? AND project_id = ?", cardID, project.ID).First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}

	var req struct {
		ColumnID uint    `json:"column_id" binding:"required"`
		Position float64 `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var col models.Column
	if err := database.DB.Where("id = ? AND project_id = ?", req.ColumnID, project.ID).First(&col).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "column not found in project"})
		return
	}

	pos := req.Position
	if pos == 0 {
		var maxPos struct{ Pos float64 }
		database.DB.Model(&models.Card{}).Select("COALESCE(MAX(position), 0) as pos").Where("column_id = ?", col.ID).Scan(&maxPos)
		pos = maxPos.Pos + 1000
	}

	oldColumnID := card.ColumnID
	database.DB.Model(&card).Updates(map[string]interface{}{"column_id": col.ID, "position": pos})

	if oldColumnID != col.ID {
		database.DB.Create(&models.CardHistory{
			CardID:       card.ID,
			UserID:       userID,
			EventType:    "column_move",
			FromColumnID: &oldColumnID,
			ToColumnID:   &col.ID,
		})
	}

	appws.BroadcastToProject(project.ID, appws.Message{
		Type: appws.TypeBoardCardMoved,
		Payload: map[string]interface{}{
			"card_id":       card.ID,
			"from_column_id": oldColumnID,
			"to_column_id":  col.ID,
			"position":      pos,
		},
	})

	database.DB.Preload("CreatedBy").Preload("Assignee").Preload("Labels").First(&card, card.ID)
	c.JSON(http.StatusOK, card)
}

// ticketAPICard is a Card enriched with the name of its column and its
// human-readable key (e.g. "ANSI-12"), so API clients can filter by lane
// name without a separate column lookup.
type ticketAPICard struct {
	models.Card
	ColumnName string `json:"column_name"`
	Key        string `json:"key"`
}

// ticketAPIProject resolves the project from the path and checks that the
// API key's user has at least viewer access. Writes the error response and
// returns nil on failure.
func ticketAPIProject(c *gin.Context) *models.Project {
	project, err := services.GetProjectBySlug(c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return nil
	}
	if err := services.RequireProjectRole(project.ID, middleware.GetUserID(c), middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil
	}
	return project
}

func ticketAPICardKey(project *models.Project, card *models.Card) string {
	if project.KeyPrefix == "" || card.CardNumber == 0 {
		return ""
	}
	return project.KeyPrefix + "-" + strconv.Itoa(card.CardNumber)
}

// TicketListColumns godoc
// @Summary      List a project's columns (lanes) via API key
// @Tags         ticket
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Success      200 {array} models.Column
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/columns [get]
func TicketListColumns(c *gin.Context) {
	project := ticketAPIProject(c)
	if project == nil {
		return
	}
	var cols []models.Column
	database.DB.Where("project_id = ?", project.ID).Order("position ASC").Find(&cols)
	c.JSON(http.StatusOK, cols)
}

// TicketListCards godoc
// @Summary      List a project's cards via API key (read-only)
// @Description  Returns open cards by default, ordered by column then position.
// @Description  Filter by lane with column_id or column (case-insensitive name).
// @Tags         ticket
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug    path  string true  "Project slug"
// @Param        column_id      query int    false "Only cards in this column"
// @Param        column         query string false "Only cards in the column with this name (case-insensitive)"
// @Param        include_closed query bool   false "Also return closed cards"
// @Success      200 {array} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards [get]
func TicketListCards(c *gin.Context) {
	project := ticketAPIProject(c)
	if project == nil {
		return
	}

	var cols []models.Column
	database.DB.Where("project_id = ?", project.ID).Order("position ASC").Find(&cols)
	colNames := make(map[uint]string, len(cols))
	colOrder := make(map[uint]int, len(cols))
	for i, col := range cols {
		colNames[col.ID] = col.Name
		colOrder[col.ID] = i
	}

	q := database.DB.Where("project_id = ?", project.ID)
	if raw := c.Query("column_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid column_id"})
			return
		}
		if _, ok := colNames[uint(id)]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "column not found in project"})
			return
		}
		q = q.Where("column_id = ?", id)
	}
	if name := strings.TrimSpace(c.Query("column")); name != "" {
		var ids []uint
		for _, col := range cols {
			if strings.EqualFold(col.Name, name) {
				ids = append(ids, col.ID)
			}
		}
		if len(ids) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "column not found in project"})
			return
		}
		q = q.Where("column_id IN ?", ids)
	}
	if c.Query("include_closed") != "true" {
		q = q.Where("closed = ?", false)
	}

	var cards []models.Card
	if err := q.Preload("CreatedBy").Preload("Assignee").Preload("Assignees").
		Preload("Labels").Preload("Tags").Preload("Epic").
		Order("position ASC").Find(&cards).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	sort.SliceStable(cards, func(i, j int) bool {
		return colOrder[cards[i].ColumnID] < colOrder[cards[j].ColumnID]
	})

	out := make([]ticketAPICard, len(cards))
	for i := range cards {
		out[i] = ticketAPICard{Card: cards[i], ColumnName: colNames[cards[i].ColumnID], Key: ticketAPICardKey(project, &cards[i])}
	}
	c.JSON(http.StatusOK, out)
}

// TicketGetCard godoc
// @Summary      Get a single card via API key (read-only)
// @Tags         ticket
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId      path int    true "Card ID"
// @Success      200 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId} [get]
func TicketGetCard(c *gin.Context) {
	project := ticketAPIProject(c)
	if project == nil {
		return
	}
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}
	var card models.Card
	if err := database.DB.Where("id = ? AND project_id = ?", cardID, project.ID).
		Preload("CreatedBy").Preload("Assignee").Preload("Assignees").
		Preload("Labels").Preload("Tags").Preload("Epic").
		Preload("Comments", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		Preload("Comments.User").
		First(&card).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}
	var col models.Column
	database.DB.Select("name").First(&col, card.ColumnID)
	c.JSON(http.StatusOK, ticketAPICard{Card: card, ColumnName: col.Name, Key: ticketAPICardKey(project, &card)})
}
