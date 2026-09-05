package api

import (
	"errors"
	"net/http"
	"strings"

	"ledger/stats"

	"github.com/gin-gonic/gin"
)

func showGameLeaderboard(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		metric := c.DefaultQuery("metric", "wins")

		entries, err := service.GameLeaderboard(c.Request.Context(), c.Param("key"), metric, leaderboardLimit(c))

		if err != nil {
			if errors.Is(err, stats.ErrUnknownMetric) {
				Fail(c, http.StatusBadRequest, "invalid_metric",
					"metric must be one of "+strings.Join(stats.GameMetrics(), ", "))

				return
			}

			failStats(c, err)

			return
		}

		OK(c, entries)
	}
}
