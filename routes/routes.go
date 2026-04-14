package routes

import (
	"database/sql"
	_ "embed"
	"ledger/handlers"
	"ledger/middleware"
	"ledger/perms"
	"net/http"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openapiSpec []byte

func SetupRoutes(r *gin.Engine, enforcer *casbin.Enforcer, db *sql.DB) {
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", openapiSpec)
	})

	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	user := v1.Group("/user")
	user.POST("/login", middleware.NotAuthenticated, handlers.Login(db))
	user.POST("/session/refresh", middleware.SessionRequired(db), handlers.RefreshSession(db))
	user.POST("/token", middleware.AuthRequired(enforcer, db, perms.UserCreateToken), handlers.CreateToken(db, enforcer))
	user.GET("/tokens", middleware.AuthRequired(enforcer, db, perms.UserCreateToken), handlers.ListTokens(db))
	user.DELETE("/tokens/:id", middleware.AuthRequired(enforcer, db, perms.UserCreateToken), handlers.RevokeToken(db))
	user.GET("", middleware.AuthRequired(enforcer, db, perms.UserRead), handlers.GetUser(db))
	user.GET("/permissions", middleware.AuthRequired(enforcer, db, perms.UserRead), handlers.GetUserPermissions(enforcer))
	user.PUT("/password", middleware.SessionRequired(db), handlers.ChangePassword(db))
	user.PUT("/username", middleware.SessionRequired(db), handlers.UpdateUsername(db))
	user.PUT("/email", middleware.SessionRequired(db), handlers.UpdateEmail(db))

	users := v1.Group("/users")
	users.GET("", middleware.AuthRequired(enforcer, db, perms.UsersRead), handlers.ListUsers(db))
	users.POST("", middleware.AuthRequired(enforcer, db, perms.UsersCreate), handlers.CreateUser(db, enforcer))
	users.DELETE("/:id", middleware.AuthRequired(enforcer, db, perms.UsersCreate), handlers.DeleteUser(db, enforcer))

	v1.POST("/auth/verify-invitation", middleware.NotAuthenticated, handlers.VerifyInvitation(db))

	roles := v1.Group("/roles")
	roles.GET("", middleware.AuthRequired(enforcer, db, perms.RolesRead), handlers.ListRoles(db, enforcer))
	roles.POST("", middleware.AuthRequired(enforcer, db, perms.RolesCreate), handlers.CreateRole(db))
	roles.DELETE("/:role", middleware.AuthRequired(enforcer, db, perms.RolesCreate), handlers.DeleteRole(db, enforcer))
	roles.GET("/:role/permissions", middleware.AuthRequired(enforcer, db, perms.RolesRead), handlers.ListRolePermissions(enforcer))
	roles.POST("/:role/permissions", middleware.AuthRequired(enforcer, db, perms.RolesCreate), handlers.AddRolePermission(enforcer))
	roles.DELETE("/:role/permissions", middleware.AuthRequired(enforcer, db, perms.RolesCreate), handlers.RemoveRolePermission(enforcer))
	roles.POST("/:role/users", middleware.AuthRequired(enforcer, db, perms.RolesManageUsers), handlers.AddUserToRole(db, enforcer))
	roles.DELETE("/:role/users", middleware.AuthRequired(enforcer, db, perms.RolesManageUsers), handlers.RemoveUserFromRole(db, enforcer))

}
