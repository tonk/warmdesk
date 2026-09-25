package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	appws "github.com/tonk/warmdesk/ws"
	"gorm.io/gorm"
)

// ticketAPICard is a Card enriched with the name of its column and its
// human-readable key (e.g. "ANSI-12"), so API clients can filter by lane
// name without a separate column lookup.
type ticketAPICard struct {
	models.Card
	ColumnName string `json:"column_name"`
	Key        string `json:"key"`
}

// ticketAPIPriorities are the priority values a card accepts.
var ticketAPIPriorities = map[string]bool{"none": true, "low": true, "medium": true, "high": true, "critical": true}

// ticketAPIProject resolves the project from the path and checks that the
// API key's user has at least minRole access. Writes the error response and
// returns nil on failure.
func ticketAPIProject(c *gin.Context, minRole string) *models.Project {
	project, err := services.GetProjectBySlug(c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return nil
	}
	if err := services.RequireProjectRole(project.ID, middleware.GetUserID(c), middleware.GetGlobalRole(c), minRole); err != nil {
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

// ticketAPIFindCard loads the card named by the :cardId path parameter, which
// is either the numeric card id or the card's key ("ANSI-12", prefix
// case-insensitive). A former key of a card that moved into this project
// (see models.CardKeyAlias) resolves too. Cards of other projects are never
// found; for a card that moved away the 404 names its new key when the caller
// may see it. Writes the error response and returns nil on failure.
func ticketAPIFindCard(c *gin.Context, project *models.Project) *models.Card {
	ref := c.Param("cardId")
	var card models.Card
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		if err := database.DB.Where("project_id = ? AND id = ?", project.ID, id).First(&card).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
			return nil
		}
		return &card
	}
	prefix, num, ok := services.ParseCardKey(ref)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return nil
	}
	if prefix == strings.ToUpper(project.KeyPrefix) &&
		database.DB.Where("project_id = ? AND card_number = ?", project.ID, num).First(&card).Error == nil {
		return &card
	}
	moved, err := services.FindCardByAlias(prefix, num)
	if err == nil && moved.ProjectID == project.ID {
		return moved
	}
	resp := gin.H{"error": "card not found"}
	if err == nil && middleware.APIKeyAllowsProject(c, moved.ProjectID) &&
		services.RequireProjectRole(moved.ProjectID, middleware.GetUserID(c), middleware.GetGlobalRole(c), "viewer") == nil {
		var target models.Project
		if database.DB.Select("key_prefix, slug").First(&target, moved.ProjectID).Error == nil {
			resp["error"] = "card moved to another project"
			resp["moved_to"] = ticketAPICardKey(&target, moved)
			resp["project"] = target.Slug
		}
	}
	c.JSON(http.StatusNotFound, resp)
	return nil
}

// ticketAPIColumn resolves the target column from a request body's
// "column_id" (number) or "column" (name, case-insensitive) field. Returns
// (nil, "") when neither is present, or an error message for a 400.
func ticketAPIColumn(project *models.Project, body map[string]json.RawMessage) (*models.Column, string) {
	rawID, hasID := body["column_id"]
	rawName, hasName := body["column"]
	if hasID && hasName {
		return nil, "give column_id or column, not both"
	}
	var col models.Column
	switch {
	case hasID:
		var id uint
		if json.Unmarshal(rawID, &id) != nil || id == 0 {
			return nil, "column_id must be a positive integer"
		}
		if database.DB.Where("id = ? AND project_id = ?", id, project.ID).First(&col).Error != nil {
			return nil, "column not found in project"
		}
	case hasName:
		var name string
		if json.Unmarshal(rawName, &name) != nil || strings.TrimSpace(name) == "" {
			return nil, "column must be a non-empty string"
		}
		if database.DB.Where("project_id = ? AND LOWER(name) = LOWER(?)", project.ID, strings.TrimSpace(name)).
			Order("position ASC").First(&col).Error != nil {
			return nil, "column not found in project"
		}
	default:
		return nil, ""
	}
	return &col, ""
}

// parseTicketAPIDate accepts "YYYY-MM-DD" or an RFC 3339 timestamp. A plain
// date is stored as midnight UTC, the same as the web UI's card editor does.
func parseTicketAPIDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

// ticketAPICardFields validates the editable card fields in body against the
// card's current values and returns the column updates plus the history
// events for fields that actually change. Keys listed in passthrough are
// skipped (handled by the caller); any other unknown key is an error when
// strict is set and ignored otherwise. A non-empty msg means a 400; nothing
// has been written at that point.
func ticketAPICardFields(body map[string]json.RawMessage, card *models.Card, passthrough map[string]bool, strict bool) (updates map[string]interface{}, history []models.CardHistory, msg string) {
	updates = map[string]interface{}{}
	isNull := func(raw json.RawMessage) bool { return string(raw) == "null" }
	record := func(eventType, detail string) {
		history = append(history, models.CardHistory{CardID: card.ID, EventType: eventType, Detail: detail})
	}
	newStart, newDue := card.StartDate, card.DueDate

	for key, raw := range body {
		if passthrough[key] {
			continue
		}
		switch key {
		case "title":
			var s string
			if isNull(raw) || json.Unmarshal(raw, &s) != nil || strings.TrimSpace(s) == "" {
				return nil, nil, "title must be a non-empty string"
			}
			if s != card.Title {
				updates["title"] = s
				record("title_changed", s)
			}
		case "description":
			var s string
			if isNull(raw) || json.Unmarshal(raw, &s) != nil {
				return nil, nil, "description must be a string"
			}
			if s != card.Description {
				updates["description"] = s
				record("description_changed", "")
			}
		case "priority":
			var s string
			if isNull(raw) || json.Unmarshal(raw, &s) != nil || !ticketAPIPriorities[s] {
				return nil, nil, "priority must be one of: none, low, medium, high, critical"
			}
			if s != card.Priority {
				updates["priority"] = s
				record("priority_changed", s)
			}
		case "closed":
			var b bool
			if isNull(raw) || json.Unmarshal(raw, &b) != nil {
				return nil, nil, "closed must be true or false"
			}
			if b != card.Closed {
				updates["closed"] = b
				if b {
					updates["closed_at"] = time.Now()
					record("closed", "")
				} else {
					updates["closed_at"] = nil
					record("reopened", "")
				}
			}
		case "start_date", "due_date":
			var t *time.Time
			if !isNull(raw) {
				var s string
				if json.Unmarshal(raw, &s) != nil {
					return nil, nil, key + " must be a date string (YYYY-MM-DD or RFC 3339) or null"
				}
				parsed, err := parseTicketAPIDate(s)
				if err != nil {
					return nil, nil, key + " must be a date string (YYYY-MM-DD or RFC 3339) or null"
				}
				t = &parsed
			}
			if key == "start_date" {
				newStart = t
			} else {
				newDue = t
			}
		case "story_points":
			if isNull(raw) {
				updates["story_points"] = nil
				continue
			}
			var n int
			if json.Unmarshal(raw, &n) != nil || n < 0 {
				return nil, nil, "story_points must be a non-negative integer or null"
			}
			updates["story_points"] = n
		default:
			if strict {
				return nil, nil, "unknown field: " + key
			}
		}
	}

	if newStart != nil && newDue != nil && newDue.Before(*newStart) {
		return nil, nil, "due_date must not be before start_date"
	}
	dateStr := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format("2006-01-02")
	}
	for _, d := range []struct {
		key, event string
		old, new   *time.Time
	}{
		{"start_date", "start_date_changed", card.StartDate, newStart},
		{"due_date", "due_date_changed", card.DueDate, newDue},
	} {
		if _, sent := body[d.key]; !sent {
			continue
		}
		if d.new == nil {
			updates[d.key] = nil
		} else {
			updates[d.key] = *d.new
		}
		if dateStr(d.old) != dateStr(d.new) {
			detail := dateStr(d.new)
			if detail == "" {
				detail = "cleared"
			}
			record(d.event, detail)
		}
	}
	return updates, history, ""
}

// ticketAPIRespond reloads the card with its associations and writes it in
// the ticketAPICard shape.
func ticketAPIRespond(c *gin.Context, status int, project *models.Project, card *models.Card, withComments bool) {
	q := database.DB.Preload("CreatedBy").Preload("Assignee").Preload("Assignees").
		Preload("Labels").Preload("Tags").Preload("Epic")
	if withComments {
		q = q.Preload("Comments", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
			Preload("Comments.User")
	}
	q.First(card, card.ID)
	fillSubCardCounts(card)
	var col models.Column
	database.DB.Select("name").First(&col, card.ColumnID)
	c.JSON(status, ticketAPICard{Card: *card, ColumnName: col.Name, Key: ticketAPICardKey(project, card)})
}

// ticketAPIBody decodes the request body into raw fields so an absent key can
// be told apart from an explicit null. Writes a 400 and returns nil on failure.
func ticketAPIBody(c *gin.Context) map[string]json.RawMessage {
	var body map[string]json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil || body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return nil
	}
	return body
}

// TicketAdd godoc
// @Summary      Create a card via API key (CI/CD integration)
// @Description  Requires title and one of column_id / column (lane name, case-insensitive).
// @Description  Optional: description, priority, start_date, due_date, story_points.
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        body body map[string]interface{} true "Card details (title and column_id or column required)"
// @Success      201 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards [post]
func TicketAdd(c *gin.Context) {
	userID := middleware.GetUserID(c)
	project := ticketAPIProject(c, "member")
	if project == nil {
		return
	}
	body := ticketAPIBody(c)
	if body == nil {
		return
	}

	col, msg := ticketAPIColumn(project, body)
	if msg == "" && col == nil {
		msg = "column_id or column is required"
	}
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if _, ok := body["title"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title must be a non-empty string"})
		return
	}
	// Validate against a blank card; unknown fields are ignored on create, as
	// they always have been, so existing CI callers keep working.
	draft := models.Card{Priority: "none"}
	updates, _, msg := ticketAPICardFields(body, &draft, map[string]bool{"column_id": true, "column": true}, false)
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
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
		Title:       updates["title"].(string),
		Position:    maxPos.Pos + 1000,
		CreatedByID: userID,
		CardNumber:  updatedProject.CardCounter,
	}
	delete(updates, "title")
	if err := database.DB.Create(&card).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if len(updates) > 0 {
		database.DB.Model(&card).Updates(updates)
	}
	database.DB.Create(&models.CardHistory{CardID: card.ID, UserID: userID, EventType: "created"})
	database.DB.Preload("CreatedBy").Preload("Assignee").Preload("Labels").Preload("Tags").First(&card, card.ID)

	appws.BroadcastToProject(project.ID, appws.Message{Type: appws.TypeBoardCardCreated, Payload: card})
	ticketAPIRespond(c, http.StatusCreated, project, &card, false)
}

// TicketComment godoc
// @Summary      Add a comment to a card via API key
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path string true "Card ID or key (e.g. ANSI-12)"
// @Param        body body map[string]string true "Comment body"
// @Success      201 {object} models.CardComment
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId}/comments [post]
func TicketComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	project := ticketAPIProject(c, "viewer")
	if project == nil {
		return
	}
	card := ticketAPIFindCard(c, project)
	if card == nil {
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

// TicketMove godoc
// @Summary      Move a card to a different column via API key
// @Description  Target lane by column_id or column (name, case-insensitive); optional position.
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path string true "Card ID or key (e.g. ANSI-12)"
// @Param        body body map[string]interface{} true "column_id or column, and optional position"
// @Success      200 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId}/move [patch]
func TicketMove(c *gin.Context) {
	userID := middleware.GetUserID(c)
	project := ticketAPIProject(c, "member")
	if project == nil {
		return
	}
	card := ticketAPIFindCard(c, project)
	if card == nil {
		return
	}
	body := ticketAPIBody(c)
	if body == nil {
		return
	}

	col, msg := ticketAPIColumn(project, body)
	if msg == "" && col == nil {
		msg = "column_id or column is required"
	}
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	var pos float64
	if raw, ok := body["position"]; ok && string(raw) != "null" {
		if json.Unmarshal(raw, &pos) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "position must be a number"})
			return
		}
	}
	if pos == 0 {
		var maxPos struct{ Pos float64 }
		database.DB.Model(&models.Card{}).Select("COALESCE(MAX(position), 0) as pos").Where("column_id = ?", col.ID).Scan(&maxPos)
		pos = maxPos.Pos + 1000
	}

	oldColumnID := card.ColumnID
	database.DB.Model(card).Updates(map[string]interface{}{"column_id": col.ID, "position": pos})

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
			"card_id":        card.ID,
			"from_column_id": oldColumnID,
			"to_column_id":   col.ID,
			"position":       pos,
		},
	})

	ticketAPIRespond(c, http.StatusOK, project, card, false)
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
	project := ticketAPIProject(c, "viewer")
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
	project := ticketAPIProject(c, "viewer")
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
// @Param        cardId      path string true "Card ID or key (e.g. ANSI-12)"
// @Success      200 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId} [get]
func TicketGetCard(c *gin.Context) {
	project := ticketAPIProject(c, "viewer")
	if project == nil {
		return
	}
	card := ticketAPIFindCard(c, project)
	if card == nil {
		return
	}
	ticketAPIRespond(c, http.StatusOK, project, card, true)
}

// TicketUpdateCard godoc
// @Summary      Partially update a card via API key
// @Description  Only fields present in the body are changed; an explicit null clears
// @Description  start_date, due_date, or story_points. Unknown fields are rejected.
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path string true "Card ID or key (e.g. ANSI-12)"
// @Param        body body map[string]interface{} true "Any of: title, description, priority, closed, start_date, due_date, story_points"
// @Success      200 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId} [patch]
func TicketUpdateCard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	project := ticketAPIProject(c, "member")
	if project == nil {
		return
	}
	card := ticketAPIFindCard(c, project)
	if card == nil {
		return
	}
	body := ticketAPIBody(c)
	if body == nil {
		return
	}

	updates, history, msg := ticketAPICardFields(body, card, nil, true)
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	if len(updates) > 0 {
		// Model().Updates() bumps updated_at.
		if err := database.DB.Model(card).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		for i := range history {
			history[i].UserID = userID
			database.DB.Create(&history[i])
		}
	}

	ticketAPIRespond(c, http.StatusOK, project, card, true)
	if len(updates) > 0 {
		appws.BroadcastToProject(project.ID, appws.Message{Type: appws.TypeBoardCardUpdated, Payload: *card})
	}
}
