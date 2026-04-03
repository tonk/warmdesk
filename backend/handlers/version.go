package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var serverVersion = "dev"

// SetVersion is called by main to inject the build-time version string.
func SetVersion(v string) { serverVersion = v }

// GetVersion returns the server version.
//
//	@Summary	Server version
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Router		/version [get]
func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": serverVersion})
}
