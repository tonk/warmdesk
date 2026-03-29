package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/middleware"
	"github.com/tonk/coworker/models"
	"github.com/tonk/coworker/services"
)

func AdminCreateProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	project := models.Project{
		Name:        req.Name,
		Slug:        services.GenerateSlug(req.Name),
		Description: req.Description,
		Color:       req.Color,
		CreatedByID: userID,
	}
	if err := database.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "project name or slug already exists"})
		return
	}
	// Add creator as owner
	database.DB.Create(&models.ProjectMember{ProjectID: project.ID, UserID: userID, Role: "owner"})

	// Default columns from system settings
	for i, name := range getDefaultColumnNames() {
		database.DB.Create(&models.Column{ProjectID: project.ID, Name: name, Position: float64((i + 1) * 1000)})
	}

	database.DB.Preload("CreatedBy").First(&project, project.ID)
	c.JSON(http.StatusCreated, project)
}

func AdminListProjects(c *gin.Context) {
	var projects []models.Project
	database.DB.Unscoped().Preload("CreatedBy").Find(&projects)
	c.JSON(http.StatusOK, projects)
}

func AdminUpdateProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		IsArchived  *bool  `json:"is_archived"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}
	if req.IsArchived != nil {
		updates["is_archived"] = *req.IsArchived
	}

	if err := database.DB.Model(&models.Project{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var project models.Project
	database.DB.Preload("CreatedBy").First(&project, id)
	c.JSON(http.StatusOK, project)
}

func AdminDeleteProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	database.DB.Delete(&models.Project{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
