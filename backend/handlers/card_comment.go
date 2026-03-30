package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/middleware"
	"github.com/tonk/coworker/models"
	"github.com/tonk/coworker/services"
	"github.com/tonk/coworker/ws"
)

// ListComments godoc
// @Summary      List comments on a card
// @Tags         comments
// @Produce      json
// @Security     BearerAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path int true "Card ID"
// @Success      200 {array}  models.CardComment
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /projects/{projectSlug}/cards/{cardId}/comments [get]
func ListComments(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var comments []models.CardComment
	database.DB.Preload("User").Where("card_id = ?", cardID).Order("created_at asc").Find(&comments)
	c.JSON(http.StatusOK, comments)
}

// CreateComment godoc
// @Summary      Add a comment to a card
// @Tags         comments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        projectSlug path string true "Project slug"
// @Param        cardId path int true "Card ID"
// @Param        body body map[string]string true "Comment body"
// @Success      201 {object} models.CardComment
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /projects/{projectSlug}/cards/{cardId}/comments [post]
func CreateComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")
	cardID, err := strconv.ParseUint(c.Param("cardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid card id"})
		return
	}

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
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment := models.CardComment{
		CardID: uint(cardID),
		UserID: userID,
		Body:   req.Body,
	}
	database.DB.Create(&comment)
	database.DB.Preload("User").First(&comment, comment.ID)

	ws.BroadcastToProject(project.ID, ws.Message{Type: ws.TypeBoardCommentCreated, Payload: comment})

	if notifSvc != nil {
		go notifSvc.NotifyMentions(req.Body, userID, "card comment")
	}

	c.JSON(http.StatusCreated, comment)
}

func UpdateComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")
	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var comment models.CardComment
	if err := database.DB.First(&comment, commentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
		return
	}

	if comment.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "can only edit your own comments"})
		return
	}

	var req struct {
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	database.DB.Model(&comment).Updates(map[string]interface{}{"body": req.Body, "is_edited": true})
	database.DB.Preload("User").First(&comment, comment.ID)

	ws.BroadcastToProject(project.ID, ws.Message{Type: ws.TypeBoardCommentUpdated, Payload: comment})
	c.JSON(http.StatusOK, comment)
}

func DeleteComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")
	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	if err := services.RequireProjectRole(project.ID, userID, middleware.GetGlobalRole(c), "viewer"); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var comment models.CardComment
	if err := database.DB.First(&comment, commentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
		return
	}

	// Own comment or project owner
	if comment.UserID != userID {
		role := services.GetMemberRole(project.ID, userID)
		if role != "owner" && middleware.GetGlobalRole(c) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	database.DB.Delete(&comment)
	ws.BroadcastToProject(project.ID, ws.Message{
		Type:    ws.TypeBoardCommentDeleted,
		Payload: map[string]uint{"comment_id": uint(commentID), "card_id": comment.CardID},
	})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
