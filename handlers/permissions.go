package handlers

import (
	"ledger/perms"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"permissions": perms.All})
	}
}
