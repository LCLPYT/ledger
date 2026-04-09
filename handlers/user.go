package handlers

import (
	"database/sql"
	"errors"
	"ledger/auth"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	casbinlog "github.com/casbin/casbin/v2/log"
	"github.com/gin-gonic/gin"
)

func Login(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var userID int64
		var passwordHash []byte
		err := db.QueryRow(
			"SELECT id, password_hash FROM users WHERE username = $1 OR email = $1",
			req.Identifier,
		).Scan(&userID, &passwordHash)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid username or email"})
			return
		}

		err = util.VerifyPassword(passwordHash, []byte(req.Password))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password"})
			return
		}

		userIDStr := strconv.FormatInt(userID, 10)

		rawPerms, err := enforcer.GetImplicitPermissionsForUser(userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch permissions"})
			return
		}

		scopes := make([]string, 0, len(rawPerms))
		for _, p := range rawPerms {
			if len(p) >= 3 {
				scopes = append(scopes, p[1]+"."+p[2])
			}
		}

		token, err := auth.GenerateToken(userIDStr, scopes, time.Now().Add(7*24*time.Hour), db, "session")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func CreateToken(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.TokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		maxExpiry := time.Now().AddDate(1, 0, 0)
		if req.Expiry.After(maxExpiry) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expiry exceeds 1 year limit"})
			return
		}

		userID := c.GetString("userID")

		for _, scope := range req.Scopes {
			parts := strings.SplitN(scope, ".", 2)
			if len(parts) != 2 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope format: " + scope})
				return
			}
			ok, err := enforcer.Enforce(userID, parts[0], parts[1], scope)
			if err != nil || !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions for scope: " + scope})
				return
			}
		}

		token, err := auth.GenerateToken(userID, req.Scopes, req.Expiry, db, req.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func GetUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var user models.User

		err := db.QueryRow(
			"SELECT id, username, email, created_at FROM users WHERE id = $1",
			userID,
		).Scan(&user.ID, &user.Username, &user.Email, &user.Created)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}

			casbinlog.LogError(err, "Failed to query user")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}
