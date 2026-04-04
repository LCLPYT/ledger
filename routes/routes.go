package routes

import (
	"database/sql"
	"ledger/handlers"
	"ledger/middleware"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, enforcer *casbin.Enforcer, db *sql.DB) {
	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	user := v1.Group("/user")
	user.POST("/login", handlers.Login(db))
	user.POST("/token", middleware.AuthRequired(enforcer, "user.create_token"), handlers.CreateToken)
	user.GET("", middleware.AuthRequired(enforcer, "user.read"), handlers.GetUser(db))
}
