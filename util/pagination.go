package util

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParsePagination reads and clamps limit/offset query parameters.
func ParsePagination(c *gin.Context) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return
}
