package routes

import (
	"database/sql"
	_ "embed"
	"ledger/handlers"
	"ledger/middleware"
	"ledger/permissions"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openapiSpec []byte

func SetupRoutes(r *gin.Engine, enforcer *casbin.Enforcer, db *sql.DB) {
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", openapiSpec)
	})

	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	user := v1.Group("/user")
	user.POST("/login", handlers.Login(db, enforcer))
	user.POST("/token", middleware.AuthRequired(enforcer, db, permissions.UserCreateToken), handlers.CreateToken(db, enforcer))
	user.GET("", middleware.AuthRequired(enforcer, db, permissions.UserRead), handlers.GetUser(db))

	roles := v1.Group("/roles")
	roles.POST("", middleware.AuthRequired(enforcer, db, permissions.RolesCreate), handlers.CreateRole(db))
	roles.GET("/permissions", middleware.AuthRequired(enforcer, db, permissions.RolesRead), handlers.ListPermissions())
	roles.POST("/:role/users", middleware.AuthRequired(enforcer, db, permissions.RolesManageUsers), handlers.AddUserToRole(db, enforcer))
	roles.DELETE("/:role/users", middleware.AuthRequired(enforcer, db, permissions.RolesManageUsers), handlers.RemoveUserFromRole(db, enforcer))
}
