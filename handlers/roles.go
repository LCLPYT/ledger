package handlers

import (
	"database/sql"
	"errors"
	"ledger/models"
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

func CreateRole(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var role models.Role
		err := db.QueryRow(
			"INSERT INTO roles (name) VALUES ($1) RETURNING id, name, created_at",
			req.Name,
		).Scan(&role.ID, &role.Name, &role.CreatedAt)
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
