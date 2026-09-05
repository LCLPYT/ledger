package api

import (
	"ledger/stats"

	"github.com/gin-gonic/gin"
)

const (
	defaultLeaderboardLimit = 25
	maxLeaderboardLimit     = 100
)

func leaderboardLimit(c *gin.Context) int {
	limit := queryInt(c, "limit", defaultLeaderboardLimit)

	if limit < 1 {
		limit = defaultLeaderboardLimit
	}

	if limit > maxLeaderboardLimit {
		limit = maxLeaderboardLimit
	}

	return limit
}

func showStatLeaderboard(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := service.StatLeaderboard(
			c.Request.Context(), c.Param("key"), c.Param("stat"), leaderboardLimit(c))

		if err != nil {
			failStats(c, err)
			return
		}

		OK(c, entries)
	}
}
