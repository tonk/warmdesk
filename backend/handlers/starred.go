package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/middleware"
	"github.com/tonk/coworker/models"
	"github.com/tonk/coworker/services"
)

func StarProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	star := models.StarredProject{UserID: userID, ProjectID: project.ID}
	database.DB.FirstOrCreate(&star)
	c.JSON(http.StatusOK, gin.H{"starred": true})
}

func UnstarProject(c *gin.Context) {
	userID := middleware.GetUserID(c)
	slug := c.Param("projectSlug")

	project, err := services.GetProjectBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	database.DB.Delete(&models.StarredProject{}, "user_id = ? AND project_id = ?", userID, project.ID)
	c.JSON(http.StatusOK, gin.H{"starred": false})
}

func ListStarredProjects(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var stars []models.StarredProject
	database.DB.Where("user_id = ?", userID).Find(&stars)

	projectIDs := make([]uint, 0, len(stars))
	for _, s := range stars {
		projectIDs = append(projectIDs, s.ProjectID)
	}

	var projects []models.Project
	if len(projectIDs) > 0 {
		database.DB.Where("id IN ?", projectIDs).Find(&projects)
	}

	c.JSON(http.StatusOK, projects)
}
