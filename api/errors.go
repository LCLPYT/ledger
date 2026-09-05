package api

import (
	"errors"
	"log"
	"net/http"

	"ledger/stats"

	"github.com/gin-gonic/gin"
)

// failStats maps a stats service error onto the response envelope.
func failStats(c *gin.Context, err error) {
	var validation *stats.ValidationError

	if errors.As(err, &validation) {
		FailWithDetails(c, http.StatusUnprocessableEntity, "validation_failed",
			"the payload does not match the registered schema", validation.Problems)

		return
	}

	var conflict *stats.ConflictError

	if errors.As(err, &conflict) {
		FailWithDetails(c, http.StatusConflict, "registration_conflict",
			"the declaration conflicts with what is already registered", conflict.Problems)

		return
	}

	if errors.Is(err, stats.ErrNotFound) {
		Fail(c, http.StatusNotFound, "not_found", "resource not found")
		return
	}

	log.Printf("request failed: %v\n", err)

	Fail(c, http.StatusInternalServerError, "internal_error", "internal server error")
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		Fail(c, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}

	return true
}

const (
	defaultPerPage = 25
	maxPerPage     = 100
)

func pagination(c *gin.Context) (page, perPage int) {
	page = queryInt(c, "page", 1)
	perPage = queryInt(c, "per_page", defaultPerPage)

	if page < 1 {
		page = 1
	}

	if perPage < 1 {
		perPage = defaultPerPage
	}

	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	return page, perPage
}
