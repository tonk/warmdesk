package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	appws "github.com/tonk/warmdesk/ws"
)

func GetOnlineUsers(c *gin.Context) {
	users := appws.GetAllOnlineUsers()
	if users == nil {
		users = []appws.PresenceUser{}
	}
	c.JSON(http.StatusOK, users)
}
