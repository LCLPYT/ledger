package handlers

import (
	"errors"
	dbsqlc "ledger/db/sqlc"
	"ledger/models"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ListRoles(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := dbsqlc.New(pool)
		rows, err := q.ListRoles(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		result := make([]models.RoleWithMembers, 0, len(rows))
		for _, r := range rows {
			members, err := enforcer.GetUsersForRole(r.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch role members"})
				return
			}
			if members == nil {
				members = []string{}
			}
			result = append(result, models.RoleWithMembers{
				Role: models.Role{
					ID:        r.ID,
					Name:      r.Name,
					CreatedAt: r.CreatedAt.Time,
					Protected: r.Protected,
				},
				Members: members,
			})
		}

		c.JSON(http.StatusOK, result)
	}
}

func CreateRole(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		r, err := q.CreateRole(c.Request.Context(), req.Name)
		if err != nil {
			if pqErr, ok := errors.AsType[*pgconn.PgError](err); ok && pqErr.Code == pgerrcode.UniqueViolation {
				c.JSON(http.StatusConflict, gin.H{"error": "role already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusCreated, models.Role{
			ID:        r.ID,
			Name:      r.Name,
			CreatedAt: r.CreatedAt.Time,
			Protected: r.Protected,
		})
	}
}

func DeleteRole(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		q := dbsqlc.New(pool)
		protected, err := q.GetRoleProtected(c.Request.Context(), roleName)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if protected {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete a protected role"})
			return
		}

		// Remove all policies where this role is the subject
		if _, err := enforcer.RemoveFilteredPolicy(0, roleName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove role policies"})
			return
		}
		// Remove all user→role memberships
		if _, err := enforcer.RemoveFilteredGroupingPolicy(1, roleName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove role memberships"})
			return
		}
		if err := enforcer.SavePolicy(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save policy"})
			return
		}

		if err := q.DeleteRole(c.Request.Context(), roleName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "role deleted"})
	}
}

func ListRolePermissions(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		policies, _ := enforcer.GetPermissionsForUser(roleName)
		perms := make([]string, 0, len(policies))
		for _, p := range policies {
			if len(p) >= 3 {
				perms = append(perms, p[1]+"."+p[2])
			}
		}

		c.JSON(http.StatusOK, perms)
	}
}

func AddRolePermission(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		if roleName == "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify permissions of the admin role"})
			return
		}

		var req models.RolePermissionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		parts := strings.SplitN(req.Permission, ".", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission format"})
			return
		}

		added, err := enforcer.AddPolicy(roleName, parts[0], parts[1])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add permission"})
			return
		}
		if !added {
			c.JSON(http.StatusConflict, gin.H{"error": "role already has this permission"})
			return
		}
		if err := enforcer.SavePolicy(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save policy"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "permission added"})
	}
}

func RemoveRolePermission(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		if roleName == "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify permissions of the admin role"})
			return
		}

		var req models.RolePermissionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		parts := strings.SplitN(req.Permission, ".", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission format"})
			return
		}

		removed, err := enforcer.RemovePolicy(roleName, parts[0], parts[1])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove permission"})
			return
		}
		if !removed {
			c.JSON(http.StatusNotFound, gin.H{"error": "role does not have this permission"})
			return
		}
		if err := enforcer.SavePolicy(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save policy"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "permission removed"})
	}
}

func AddUserToRole(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		var req models.RoleUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		exists, err := q.RoleExists(c.Request.Context(), roleName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
			return
		}

		added, err := enforcer.AddGroupingPolicy(req.UserID, roleName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign role"})
			return
		}
		if !added {
			c.JSON(http.StatusConflict, gin.H{"error": "user already has this role"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "user added to role"})
	}
}

func RemoveUserFromRole(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		if roleName == "default" {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot remove users from the default role"})
			return
		}

		var req models.RoleUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		exists, err := q.RoleExists(c.Request.Context(), roleName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
			return
		}

		removed, err := enforcer.RemoveGroupingPolicy(req.UserID, roleName)
		if err != nil && err.Error() == "policy not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "user does not have this role"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove role"})
			return
		}
		if !removed {
			c.JSON(http.StatusNotFound, gin.H{"error": "user does not have this role"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "user removed from role"})
	}
}
