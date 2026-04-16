package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ValidateUUID returns false and writes a 400 response if the string is not a valid UUID.
func ValidateUUID(c *gin.Context, raw string) bool {
	if _, err := uuid.Parse(raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return false
	}
	return true
}
