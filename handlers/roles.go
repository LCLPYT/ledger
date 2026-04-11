package handlers

import (
	"database/sql"
	"errors"
	"ledger/models"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

func ListRoles(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, name, created_at, protected FROM roles ORDER BY name")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		result := make([]models.RoleWithMembers, 0)
		for rows.Next() {
			var r models.RoleWithMembers
			if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt, &r.Protected); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			members, err := enforcer.GetUsersForRole(r.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch role members"})
				return
			}
			if members == nil {
				members = []string{}
			}
			r.Members = members
			result = append(result, r)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

func CreateRole(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var role models.Role
		err := db.QueryRow(
			"INSERT INTO roles (name) VALUES ($1) RETURNING id, name, created_at, protected",
			req.Name,
		).Scan(&role.ID, &role.Name, &role.CreatedAt, &role.Protected)
		if err != nil {
			const PgErrDuplicate = "23505"
			if pqErr, ok := errors.AsType[*pq.Error](err); ok && pqErr.Code == PgErrDuplicate {
				c.JSON(http.StatusConflict, gin.H{"error": "role already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusCreated, role)
	}
}

func DeleteRole(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		var protected bool
		err := db.QueryRow("SELECT protected FROM roles WHERE name = $1", roleName).Scan(&protected)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
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

		if _, err := db.Exec("DELETE FROM roles WHERE name = $1", roleName); err != nil {
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

func AddUserToRole(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		var req models.RoleUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)", roleName).Scan(&exists)
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

func RemoveUserFromRole(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleName := c.Param("role")

		var req models.RoleUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)", roleName).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})
			return
		}

		removed, err := enforcer.RemoveGroupingPolicy(req.UserID, roleName)
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
