package api

import (
	"ledger/stats"

	"github.com/gin-gonic/gin"
)

func showPlayer(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathUUID(c, "uuid")

		if !ok {
			return
		}

		profile, err := service.PlayerProfile(c.Request.Context(), id)

		if err != nil {
			failStats(c, err)
			return
		}

		OK(c, profile)
	}
}

func listPlayerMatches(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathUUID(c, "uuid")

		if !ok {
			return
		}

		page, perPage := pagination(c)

		matches, total, err := service.PlayerMatches(c.Request.Context(), id, perPage, (page-1)*perPage)

		if err != nil {
			failStats(c, err)
			return
		}

		Page(c, matches, page, perPage, total)
	}
}

func listPlayerRecords(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := pathUUID(c, "uuid")

		if !ok {
			return
		}

		records, err := service.PlayerRecords(c.Request.Context(), id)

		if err != nil {
			failStats(c, err)
			return
		}

		OK(c, records)
	}
}
