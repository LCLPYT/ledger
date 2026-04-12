package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"ledger/auth"
	"ledger/email"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	casbinlog "github.com/casbin/casbin/v2/log"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

func Login(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var userID int64
		var passwordHash []byte
		err := db.QueryRow(
			"SELECT id, password_hash FROM users WHERE (username = $1 OR email = $1) AND verified_at IS NOT NULL",
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

		token, err := auth.GenerateSessionToken(userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func RefreshSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		token, err := auth.GenerateSessionToken(userID)
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

func ListTokens(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		rows, err := db.Query(
			"SELECT id, name, created_at, expires_at, revoked, scopes FROM access_tokens WHERE user_id = $1 ORDER BY created_at DESC",
			userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		tokens := make([]models.AccessToken, 0)
		for rows.Next() {
			var t models.AccessToken
			var scopesJSON []byte
			if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.ExpiresAt, &t.Revoked, &scopesJSON); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			if err := json.Unmarshal(scopesJSON, &t.Scopes); err != nil {
				t.Scopes = []string{}
			}
			tokens = append(tokens, t)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, tokens)
	}
}

func RevokeToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		tokenID := c.Param("id")

		result, err := db.Exec(
			"UPDATE access_tokens SET revoked = TRUE WHERE id = $1 AND user_id = $2 AND revoked = FALSE",
			tokenID, userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		n, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found or already revoked"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "token revoked"})
	}
}

func ListUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query("SELECT id, username, email, created_at FROM users ORDER BY created_at DESC")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		users := make([]models.User, 0)
		for rows.Next() {
			var u models.User
			if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Created); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, users)
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

func CreateUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var user models.User
		err := db.QueryRow(
			"INSERT INTO users (username, email) VALUES ($1, $2) RETURNING id, username, email, created_at",
			req.Username, req.Email,
		).Scan(&user.ID, &user.Username, &user.Email, &user.Created)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}
		token := hex.EncodeToString(raw)

		_, err = db.Exec(
			"INSERT INTO user_invitations (user_id, token, expires_at) VALUES ($1, $2, now() + interval '24 hours')",
			user.ID, token,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := email.SendVerificationEmail(user.Email, user.Username, token); err != nil {
			c.JSON(http.StatusCreated, gin.H{
				"user":    user,
				"warning": "user created but verification email could not be sent",
			})
			return
		}

		c.JSON(http.StatusCreated, user)
	}
}

func SetPassword(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.SetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var invID, userID int64
		err := db.QueryRow(
			"SELECT id, user_id FROM user_invitations WHERE token = $1 AND expires_at > now() AND used_at IS NULL",
			req.Token,
		).Scan(&invID, &userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired token"})
			return
		}

		hash, err := util.HashPassword([]byte(req.Password))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "password hashing failed"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		if _, err = tx.Exec(
			"UPDATE users SET password_hash = $1, verified_at = now() WHERE id = $2",
			hash, userID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if _, err = tx.Exec(
			"UPDATE user_invitations SET used_at = now() WHERE id = $1",
			invID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password set, account activated"})
	}
}
