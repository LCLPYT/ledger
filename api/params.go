package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
)

func queryInt(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)

	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)

	if err != nil {
		return fallback
	}

	return value
}

// pathUUID reads a uuid path parameter, replying with a 400 when it is malformed.
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	value, err := uuid.FromString(c.Param(name))

	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid_uuid", c.Param(name)+" is not a valid uuid")
		return uuid.Nil, false
	}

	return value, true
}
