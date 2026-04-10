package routes

import (
	"database/sql"
	_ "embed"
	"ledger/handlers"
	"ledger/middleware"
	"ledger/perms"
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
	user.POST("/token", middleware.AuthRequired(enforcer, db, perms.UserCreateToken), handlers.CreateToken(db, enforcer))
	user.GET("", middleware.AuthRequired(enforcer, db, perms.UserRead), handlers.GetUser(db))

	roles := v1.Group("/roles")
	roles.POST("", middleware.AuthRequired(enforcer, db, perms.RolesCreate), handlers.CreateRole(db))
	roles.POST("/:role/users", middleware.AuthRequired(enforcer, db, perms.RolesManageUsers), handlers.AddUserToRole(db, enforcer))
	roles.DELETE("/:role/users", middleware.AuthRequired(enforcer, db, perms.RolesManageUsers), handlers.RemoveUserFromRole(db, enforcer))

	permissions := v1.Group("/permissions")
	permissions.GET("", middleware.AuthRequired(enforcer, db, perms.PermissionsList), handlers.ListPermissions())
}
