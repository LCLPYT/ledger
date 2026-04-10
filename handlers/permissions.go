package handlers

import (
	"ledger/permissions"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"permissions": permissions.All})
	}
}
