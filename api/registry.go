package api

import (
	"net/http"

	"ledger/stats"

	"github.com/gin-gonic/gin"
)

func registerDefinitions(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var manifest stats.Manifest

		if !bindJSON(c, &manifest) {
			return
		}

		result, err := service.Register(c.Request.Context(), manifest)

		if err != nil {
			failStats(c, err)
			return
		}

		if result.Created > 0 {
			Created(c, result)
			return
		}

		OK(c, result)
	}
}

func listDefinitions(service *stats.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := c.Query("kind")

		if kind != "" && kind != "game" && kind != "minigame" {
			Fail(c, http.StatusBadRequest, "invalid_kind", "kind must be game or minigame")
			return
		}

		definitions, err := service.ListDefinitions(c.Request.Context(), kind, c.Query("key"))

		if err != nil {
			failStats(c, err)
			return
		}

		OK(c, definitions)
	}
}
