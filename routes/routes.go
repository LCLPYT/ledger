package routes

import (
	_ "embed"
	"ledger/handlers"
	"ledger/middleware"
	"ledger/perms"
	"net/http"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed openapi.yaml
var openapiSpec []byte

func SetupRoutes(r *gin.Engine, enforcer *casbin.Enforcer, pool *pgxpool.Pool) {
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
	user.POST("/login", middleware.NotAuthenticated, handlers.Login(pool))
	user.POST("/session/refresh", middleware.SessionRequired(pool), handlers.RefreshSession(pool))
	user.POST("/token", middleware.AuthRequired(enforcer, pool, perms.UserCreateToken), handlers.CreateToken(pool, enforcer))
	user.GET("/tokens", middleware.AuthRequired(enforcer, pool, perms.UserCreateToken), handlers.ListTokens(pool))
	user.DELETE("/tokens/:id", middleware.AuthRequired(enforcer, pool, perms.UserCreateToken), handlers.RevokeToken(pool))
	user.GET("", middleware.AuthRequired(enforcer, pool, perms.UserRead), handlers.GetUser(pool))
	user.GET("/permissions", middleware.AuthRequired(enforcer, pool, perms.UserRead), handlers.GetUserPermissions(enforcer))
	user.PUT("/password", middleware.SessionRequired(pool), handlers.ChangePassword(pool))
	user.PUT("/username", middleware.SessionRequired(pool), handlers.UpdateUsername(pool))
	user.PUT("/email", middleware.SessionRequired(pool), handlers.UpdateEmail(pool))

	users := v1.Group("/users")
	users.GET("", middleware.AuthRequired(enforcer, pool, perms.UsersRead), handlers.ListUsers(pool))
	users.POST("", middleware.AuthRequired(enforcer, pool, perms.UsersCreate), handlers.CreateUser(pool, enforcer))
	users.DELETE("/:id", middleware.AuthRequired(enforcer, pool, perms.UsersCreate), handlers.DeleteUser(pool, enforcer))

	v1.POST("/auth/verify-invitation", middleware.NotAuthenticated, handlers.VerifyInvitation(pool))

	roles := v1.Group("/roles")
	roles.GET("", middleware.AuthRequired(enforcer, pool, perms.RolesRead), handlers.ListRoles(pool, enforcer))
	roles.POST("", middleware.AuthRequired(enforcer, pool, perms.RolesCreate), handlers.CreateRole(pool))
	roles.DELETE("/:role", middleware.AuthRequired(enforcer, pool, perms.RolesCreate), handlers.DeleteRole(pool, enforcer))
	roles.GET("/:role/permissions", middleware.AuthRequired(enforcer, pool, perms.RolesRead), handlers.ListRolePermissions(enforcer))
	roles.POST("/:role/permissions", middleware.AuthRequired(enforcer, pool, perms.RolesCreate), handlers.AddRolePermission(enforcer))
	roles.DELETE("/:role/permissions", middleware.AuthRequired(enforcer, pool, perms.RolesCreate), handlers.RemoveRolePermission(enforcer))
	roles.POST("/:role/users", middleware.AuthRequired(enforcer, pool, perms.RolesManageUsers), handlers.AddUserToRole(pool, enforcer))
	roles.DELETE("/:role/users", middleware.AuthRequired(enforcer, pool, perms.RolesManageUsers), handlers.RemoveUserFromRole(pool, enforcer))

	minecraft := v1.Group("/minecraft")

	players := minecraft.Group("/players")
	players.GET("", middleware.AuthRequired(enforcer, pool, perms.PlayerRead), handlers.ListPlayers(pool))
	players.GET("/lookup", middleware.AuthRequired(enforcer, pool, perms.PlayerWrite), handlers.LookupPlayerByName(pool))

	player := players.Group("/:uuid")
	player.GET("", middleware.AuthRequired(enforcer, pool, perms.PlayerWrite), handlers.GetPlayer(pool))
	player.DELETE("", middleware.AuthRequired(enforcer, pool, perms.PlayerWrite), handlers.DeletePlayer(pool))

	data := player.Group("/data")
	data.GET("", middleware.AuthRequired(enforcer, pool, perms.PlayerDataRead), handlers.GetPlayerData(pool))
	data.GET("/*path", middleware.AuthRequired(enforcer, pool, perms.PlayerDataRead), handlers.GetPlayerData(pool))
	data.PUT("/*path", middleware.AuthRequired(enforcer, pool, perms.PlayerDataWrite), handlers.SetPlayerData(pool))
	data.DELETE("/*path", middleware.AuthRequired(enforcer, pool, perms.PlayerDataWrite), handlers.DeletePlayerData(pool))

	coins := player.Group("/coins")
	coins.GET("", middleware.AuthRequired(enforcer, pool, perms.CoinsRead), handlers.GetPlayerCoins(pool))
	coins.GET("/transactions", middleware.AuthRequired(enforcer, pool, perms.CoinsRead), handlers.GetPlayerTransactions(pool))
	coins.POST("/award", middleware.AuthRequired(enforcer, pool, perms.CoinsWrite), handlers.AwardCoins(pool))
	coins.POST("/spend", middleware.AuthRequired(enforcer, pool, perms.CoinsWrite), handlers.SpendCoins(pool))
	coins.POST("/adjust", middleware.AuthRequired(enforcer, pool, perms.CoinsWrite), handlers.AdjustCoins(pool))
}
