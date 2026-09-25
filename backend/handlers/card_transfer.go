package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"github.com/tonk/warmdesk/ws"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// transferError carries the HTTP status and message of a rejected transfer.
type transferError struct {
	status int
	msg    string
}

// transferResult is the outcome of a copy or move to another project.
type transferResult struct {
	Card          *models.Card // the card in the target project
	PreviousKey   string       // key in the source project (move only)
	MovedSubCards []uint       // ids of sub-cards that moved along
}

// transferCardResponse is the REST shape of TransferCard's response.
type transferCardResponse struct {
	models.Card
	Key             string `json:"key"`
	PreviousKey     string `json:"previous_key,omitempty"`
	MovedSubCardIDs []uint `json:"moved_sub_card_ids,omitempty"`
}

// transferCard copies or moves card from source into column col of target on
// behalf of the request's user. The caller has already checked member access
// to source and that col belongs to target.
//
// A copy creates a new card with the title, description, priority, due date
// and tags. A move keeps the card itself (so comments, checklists,
// attachments, history, links and time spent come along) and changes:
//   - the number: the card gets the next number in target, and its old key
//     is kept as a models.CardKeyAlias so references to it still resolve;
//   - labels: mapped by name (case-insensitive) onto target's labels, missing
//     ones are created with the same colour;
//   - epic and sprints: cleared, as both belong to source;
//   - assignees and watchers without access to target: removed;
//   - sub-cards: moved along (subCards "move", the default) into the target
//     column of the same name, else col; or detached ("detach"). A parent in
//     source is detached from the moved card.
func transferCard(c *gin.Context, source *models.Project, card *models.Card, target *models.Project, col *models.Column, action, subCards string) (*transferResult, *transferError) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)

	if action != "copy" && action != "move" {
		return nil, &transferError{http.StatusBadRequest, "action must be 'copy' or 'move'"}
	}
	if subCards == "" {
		subCards = "move"
	}
	if subCards != "move" && subCards != "detach" {
		return nil, &transferError{http.StatusBadRequest, "sub_cards must be 'move' or 'detach'"}
	}
	// The API key scope check only covers the source project in the path.
	if !middleware.APIKeyAllowsProject(c, target.ID) {
		return nil, &transferError{http.StatusForbidden, "key not valid for target project"}
	}
	if err := services.RequireProjectRole(target.ID, userID, role, "member"); err != nil {
		return nil, &transferError{http.StatusForbidden, "forbidden in target project"}
	}

	if action == "copy" {
		return copyCardToProject(card, target, col, userID), nil
	}
	if source.ID == target.ID {
		return nil, &transferError{http.StatusBadRequest, "card is already in this project; move it to another column instead"}
	}

	// Everything read through database.DB happens before the transaction: an
	// in-memory SQLite test database has one connection per pool slot, so a
	// read outside tx while tx is open would see an empty database.
	moving := []models.Card{*card}
	if subCards == "move" {
		moving = append(moving, subCardTree(card.ID)...)
	}
	movingIDs := make([]uint, len(moving))
	for i, m := range moving {
		movingIDs[i] = m.ID
	}
	var targetCols []models.Column
	database.DB.Where("project_id = ?", target.ID).Order("position ASC").Find(&targetCols)
	columnFor := func(m models.Card) uint {
		if m.ID == card.ID {
			return col.ID
		}
		var name string
		database.DB.Model(&models.Column{}).Where("id = ?", m.ColumnID).Pluck("name", &name)
		for _, tc := range targetCols {
			if strings.EqualFold(tc.Name, name) {
				return tc.ID
			}
		}
		return col.ID
	}
	destColumn := map[uint]uint{}
	for _, m := range moving {
		destColumn[m.ID] = columnFor(m)
	}
	lostAccess := usersWithoutProjectAccess(target.ID, movingIDs)

	result := &transferResult{PreviousKey: ticketAPICardKey(source, card)}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		labelFor := map[string]uint{} // lower-cased name → target label id
		var targetLabels []models.Label
		tx.Where("project_id = ?", target.ID).Order("id ASC").Find(&targetLabels)
		for _, l := range targetLabels {
			if _, seen := labelFor[strings.ToLower(l.Name)]; !seen {
				labelFor[strings.ToLower(l.Name)] = l.ID
			}
		}

		if subCards == "detach" {
			if err := tx.Unscoped().Model(&models.Card{}).Where("parent_card_id = ?", card.ID).
				Update("parent_card_id", nil).Error; err != nil {
				return err
			}
		}

		for _, m := range moving {
			if err := tx.Model(&models.Project{}).Where("id = ?", target.ID).
				UpdateColumn("card_counter", gorm.Expr("card_counter + 1")).Error; err != nil {
				return err
			}
			var counter int
			tx.Model(&models.Project{}).Where("id = ?", target.ID).Pluck("card_counter", &counter)

			if source.KeyPrefix != "" && m.CardNumber > 0 {
				alias := models.CardKeyAlias{KeyPrefix: strings.ToUpper(source.KeyPrefix), CardNumber: m.CardNumber, CardID: m.ID}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "key_prefix"}, {Name: "card_number"}},
					DoUpdates: clause.AssignmentColumns([]string{"card_id"}),
				}).Create(&alias).Error; err != nil {
					return err
				}
			}

			var maxPos float64
			tx.Model(&models.Card{}).Where("column_id = ?", destColumn[m.ID]).
				Select("COALESCE(MAX(position), 0)").Scan(&maxPos)
			updates := map[string]interface{}{
				"project_id":  target.ID,
				"column_id":   destColumn[m.ID],
				"position":    maxPos + 1000,
				"card_number": counter,
				"epic_id":     nil,
			}
			if m.ID == card.ID {
				updates["parent_card_id"] = nil
			}
			if m.AssigneeID != nil && lostAccess[*m.AssigneeID] {
				updates["assignee_id"] = nil
			}
			// Unscoped: a soft-deleted sub-card moves along and stays deleted.
			if err := tx.Unscoped().Model(&models.Card{}).Where("id = ?", m.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.Where("card_id = ?", m.ID).Delete(&models.SprintCard{}).Error; err != nil {
				return err
			}
			if err := remapCardLabels(tx, m.ID, target.ID, labelFor); err != nil {
				return err
			}

			if err := tx.Create(&models.CardHistory{
				CardID:    m.ID,
				UserID:    userID,
				EventType: "project_moved",
				Detail:    ticketAPICardKey(source, &m) + " → " + ticketAPICardKey(target, &models.Card{CardNumber: counter}),
			}).Error; err != nil {
				return err
			}
			if m.ID != card.ID {
				result.MovedSubCards = append(result.MovedSubCards, m.ID)
			}
		}

		if len(lostAccess) > 0 {
			ids := make([]uint, 0, len(lostAccess))
			for id := range lostAccess {
				ids = append(ids, id)
			}
			if err := tx.Where("card_id IN ? AND user_id IN ?", movingIDs, ids).Delete(&models.CardAssignee{}).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM card_watchers WHERE card_id IN ? AND user_id IN ?", movingIDs, ids).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, &transferError{http.StatusInternalServerError, "failed to move card"}
	}

	for _, m := range moving {
		ws.BroadcastToProject(source.ID, ws.Message{
			Type:    ws.TypeBoardCardDeleted,
			Payload: map[string]uint{"card_id": m.ID, "column_id": m.ColumnID},
		})
		var moved models.Card
		if database.DB.Preload("Labels").Preload("Assignee").Preload("Assignees").Preload("Tags").
			First(&moved, m.ID).Error == nil {
			fillSubCardCounts(&moved)
			ws.BroadcastToProject(target.ID, ws.Message{Type: ws.TypeBoardCardCreated, Payload: moved})
			if m.ID == card.ID {
				result.Card = &moved
			}
		}
	}
	if result.Card == nil {
		// The moved card itself was soft-deleted; still report where it went.
		var moved models.Card
		database.DB.Unscoped().First(&moved, card.ID)
		result.Card = &moved
	}
	return result, nil
}

// fillSubCardCounts sets the computed SubCardCount/SubCardsDone fields, which
// are not stored columns and stay zero unless a handler fills them in.
func fillSubCardCounts(card *models.Card) {
	var total, done int64
	database.DB.Model(&models.Card{}).Where("parent_card_id = ?", card.ID).Count(&total)
	if total > 0 {
		database.DB.Model(&models.Card{}).Where("parent_card_id = ? AND closed = true", card.ID).Count(&done)
	}
	card.SubCardCount = int(total)
	card.SubCardsDone = int(done)
}

// copyCardToProject creates a copy of original at the bottom of col in target.
func copyCardToProject(original *models.Card, target *models.Project, col *models.Column, userID uint) *transferResult {
	var tags []models.CardTag
	database.DB.Where("card_id = ?", original.ID).Find(&tags)

	var maxPos float64
	database.DB.Model(&models.Card{}).Where("column_id = ?", col.ID).Select("COALESCE(MAX(position), 0)").Scan(&maxPos)

	database.DB.Model(&models.Project{}).Where("id = ?", target.ID).
		UpdateColumn("card_counter", gorm.Expr("card_counter + 1"))
	var updatedProject models.Project
	database.DB.Select("card_counter").First(&updatedProject, target.ID)

	newCard := models.Card{
		ColumnID:    col.ID,
		ProjectID:   target.ID,
		Title:       original.Title,
		Description: original.Description,
		Priority:    original.Priority,
		DueDate:     original.DueDate,
		CreatedByID: userID,
		Position:    maxPos + 1000,
		CardNumber:  updatedProject.CardCounter,
	}
	database.DB.Create(&newCard)

	for _, tag := range tags {
		database.DB.Create(&models.CardTag{CardID: newCard.ID, Name: tag.Name})
	}

	database.DB.Preload("Labels").Preload("Assignee").Preload("Assignees").Preload("Tags").First(&newCard, newCard.ID)
	ws.BroadcastToProject(target.ID, ws.Message{Type: ws.TypeBoardCardCreated, Payload: newCard})
	return &transferResult{Card: &newCard}
}

// subCardTree returns every descendant of cardID, soft-deleted ones included.
func subCardTree(cardID uint) []models.Card {
	var all []models.Card
	seen := map[uint]bool{cardID: true}
	frontier := []uint{cardID}
	for len(frontier) > 0 {
		var children []models.Card
		database.DB.Unscoped().Where("parent_card_id IN ?", frontier).Order("id ASC").Find(&children)
		frontier = nil
		for _, ch := range children {
			if seen[ch.ID] {
				continue
			}
			seen[ch.ID] = true
			all = append(all, ch)
			frontier = append(frontier, ch.ID)
		}
	}
	return all
}

// usersWithoutProjectAccess returns the assignees and watchers of cardIDs
// who cannot view projectID.
func usersWithoutProjectAccess(projectID uint, cardIDs []uint) map[uint]bool {
	var ids []uint
	var legacy, multi, watchers []uint
	database.DB.Unscoped().Model(&models.Card{}).Where("id IN ? AND assignee_id IS NOT NULL", cardIDs).Pluck("assignee_id", &legacy)
	database.DB.Model(&models.CardAssignee{}).Where("card_id IN ?", cardIDs).Pluck("user_id", &multi)
	database.DB.Table("card_watchers").Where("card_id IN ?", cardIDs).Pluck("user_id", &watchers)
	ids = append(append(append(ids, legacy...), multi...), watchers...)

	lost := map[uint]bool{}
	if len(ids) == 0 {
		return lost
	}
	var users []models.User
	database.DB.Select("id, global_role").Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		if services.RequireProjectRole(projectID, u.ID, u.GlobalRole, "viewer") != nil {
			lost[u.ID] = true
		}
	}
	return lost
}

// remapCardLabels replaces cardID's labels with the target project's labels
// of the same name, creating the ones target lacks. labelFor caches target
// labels by lower-cased name across calls.
func remapCardLabels(tx *gorm.DB, cardID, targetProjectID uint, labelFor map[string]uint) error {
	var labels []models.Label
	tx.Joins("JOIN card_labels ON card_labels.label_id = labels.id").
		Where("card_labels.card_id = ?", cardID).Find(&labels)
	if err := tx.Where("card_id = ?", cardID).Delete(&models.CardLabel{}).Error; err != nil {
		return err
	}
	done := map[uint]bool{}
	for _, l := range labels {
		key := strings.ToLower(l.Name)
		id, ok := labelFor[key]
		if !ok {
			created := models.Label{ProjectID: targetProjectID, Name: l.Name, Color: l.Color}
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			id = created.ID
			labelFor[key] = id
		}
		if done[id] {
			continue
		}
		done[id] = true
		if err := tx.Create(&models.CardLabel{CardID: cardID, LabelID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

// TransferCard godoc
// @Summary      Copy or move a card to a column in another project
// @Description  action "copy" creates a new card with the title, description, priority, due date and tags.
// @Description  action "move" keeps the card itself (comments, checklist, attachments, history, links, time spent)
// @Description  and gives it the next number in the target project; its old key (e.g. PRJ-12) keeps resolving.
// @Description  Labels are matched by name (missing ones are created), epic and sprints are cleared, and
// @Description  assignees/watchers without access to the target project are removed.
// @Description  sub_cards: "move" (default) moves sub-cards along, "detach" leaves them in the source project.
// @Description  Requires member access to both projects; a project-scoped API key must be scoped to the target.
// @Tags         cards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Source project slug"
// @Param        cardId path int true "Card ID"
// @Param        body body map[string]interface{} true "target_project_slug, column_id, action (copy|move), optional sub_cards (move|detach)"
// @Success      201 {object} transferCardResponse
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /projects/{projectSlug}/cards/{cardId}/transfer [post]
func TransferCard(c *gin.Context) {
	userID := middleware.GetUserID(c)
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}

	var req struct {
		TargetProjectSlug string `json:"target_project_slug" binding:"required"`
		ColumnID          uint   `json:"column_id" binding:"required"`
		Action            string `json:"action" binding:"required"` // "copy" or "move"
		SubCards          string `json:"sub_cards"`                 // "move" (default) or "detach"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	sourceProject, err := services.GetProjectBySlug(c.Param("projectSlug"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source project not found"})
		return
	}
	if err := services.RequireProjectRole(sourceProject.ID, userID, middleware.GetGlobalRole(c), "member"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	targetProject, err := services.GetProjectBySlug(req.TargetProjectSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target project not found"})
		return
	}
	var targetColumn models.Column
	if err := database.DB.Where("id = ? AND project_id = ?", req.ColumnID, targetProject.ID).First(&targetColumn).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "column not found in target project"})
		return
	}
	var original models.Card
	if err := database.DB.Where("id = ? AND project_id = ?", cardID, sourceProject.ID).First(&original).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found"})
		return
	}

	res, terr := transferCard(c, sourceProject, &original, targetProject, &targetColumn, req.Action, req.SubCards)
	if terr != nil {
		c.JSON(terr.status, gin.H{"error": terr.msg})
		return
	}
	c.JSON(http.StatusCreated, transferCardResponse{
		Card:            *res.Card,
		Key:             ticketAPICardKey(targetProject, res.Card),
		PreviousKey:     res.PreviousKey,
		MovedSubCardIDs: res.MovedSubCards,
	})
}

// TicketTransfer godoc
// @Summary      Move or copy a card to another project via API key
// @Description  Body: target_project (slug, required), column_id or column (lane name in the target project,
// @Description  case-insensitive, required), action "move" (default) or "copy", sub_cards "move" (default) or "detach".
// @Description  See POST /projects/{projectSlug}/cards/{cardId}/transfer for what a move changes. The response is
// @Description  the card in the target project, with its new key; the old key keeps resolving.
// @Tags         ticket
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        projectSlug path string true "Source project slug"
// @Param        cardId path string true "Card ID or key (e.g. ANSI-12)"
// @Param        body body map[string]interface{} true "target_project, column_id or column, optional action and sub_cards"
// @Success      201 {object} ticketAPICard
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /ticket/{projectSlug}/cards/{cardId}/transfer [post]
func TicketTransfer(c *gin.Context) {
	source := ticketAPIProject(c, "member")
	if source == nil {
		return
	}
	card := ticketAPIFindCard(c, source)
	if card == nil {
		return
	}
	body := ticketAPIBody(c)
	if body == nil {
		return
	}
	str := func(key string) (string, error) {
		raw, ok := body[key]
		if !ok || string(raw) == "null" {
			return "", nil
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", errors.New(key + " must be a string")
		}
		return strings.TrimSpace(s), nil
	}
	targetSlug, err1 := str("target_project")
	action, err2 := str("action")
	subCards, err3 := str("sub_cards")
	if err := errors.Join(err1, err2, err3); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for key := range body {
		switch key {
		case "target_project", "column_id", "column", "action", "sub_cards":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown field: " + key})
			return
		}
	}
	if targetSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_project is required"})
		return
	}
	if action == "" {
		action = "move"
	}
	target, err := services.GetProjectBySlug(targetSlug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target project not found"})
		return
	}
	col, msg := ticketAPIColumn(target, body)
	if msg == "" && col == nil {
		msg = "column_id or column is required"
	}
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": strings.Replace(msg, "in project", "in target project", 1)})
		return
	}

	res, terr := transferCard(c, source, card, target, col, action, subCards)
	if terr != nil {
		c.JSON(terr.status, gin.H{"error": terr.msg})
		return
	}
	ticketAPIRespond(c, http.StatusCreated, target, res.Card, false)
}
