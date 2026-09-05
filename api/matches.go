package api

import (
	"ledger/stats"

	"github.com/gin-gonic/gin"
)

func ingestMatch(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var match stats.MatchInput

		if !bindJSON(c, &match) {
			return
		}

		result, err := service.IngestMatch(c.Request.Context(), &match)

		if err != nil {
			failStats(c, err)
			return
		}

		if result.Created {
			Created(c, result)
			return
		}

		OK(c, result)
	}
}

func showMatch(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathUUID(c, "id")

		if !ok {
			return
		}

		detail, err := service.MatchDetail(c.Request.Context(), id)

		if err != nil {
			failStats(c, err)
			return
		}

		OK(c, detail)
	}
}
