package api

import (
	"ledger/stats"

	"github.com/gin-gonic/gin"
	"github.com/stephenafamo/bob"
)

func SetupRoutes(router *gin.Engine, db bob.DB) {
	router.NoRoute(NotFound)

	service := stats.NewService(db)

	v1 := router.Group("/api/v1")

	v1.GET("/health", func(c *gin.Context) {
		OK(c, gin.H{"status": "ok"})
	})

	registry := v1.Group("/registry")
	registry.POST("/definitions", registerDefinitions(service))
	registry.GET("/definitions", listDefinitions(service))

	v1.POST("/matches", ingestMatch(service))
	v1.GET("/matches/:id", showMatch(service))

	players := v1.Group("/players")
	players.GET("/:uuid", showPlayer(service))
	players.GET("/:uuid/matches", listPlayerMatches(service))
	players.GET("/:uuid/records", listPlayerRecords(service))

	v1.GET("/records/:key/:stat", showStatLeaderboard(service))
	v1.GET("/leaderboards/:key", showGameLeaderboard(service))
}
