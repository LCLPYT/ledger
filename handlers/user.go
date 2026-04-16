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
	"ledger/perms"
	"ledger/util"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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

		token, err := auth.GenerateSessionToken(userIDStr, db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func RefreshSession(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		sessionID := c.GetString("sessionID")

		expiry := time.Now().Add(auth.SessionLifetime)
		result, err := db.Exec(
			"UPDATE sessions SET expires_at = $1 WHERE id = $2 AND user_id = $3",
			expiry, sessionID, userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		n, err := result.RowsAffected()
		if err != nil || n == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session not found"})
			return
		}

		token, err := auth.GenerateSessionTokenFromID(userID, sessionID, expiry)
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

			log.Printf("Failed to query user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

func CreateUser(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
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
			if pqErr, ok := errors.AsType[*pgconn.PgError](err); ok && pqErr.Code == pgerrcode.UniqueViolation {
				c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		uid := strconv.FormatInt(user.ID, 10)
		if _, err := enforcer.AddGroupingPolicy(uid, "default"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission error"})
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

func VerifyInvitation(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.VerifyInvitationRequest
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

		if err := util.ValidatePassword(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

// verifyCurrentPassword fetches the user's password hash and checks it against
// the provided password. Returns false and writes the appropriate JSON error
// response if the check fails, so the caller can return immediately.
func verifyCurrentPassword(c *gin.Context, db *sql.DB, userID, password string) bool {
	var passwordHash []byte
	if err := db.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return false
	}
	if err := util.VerifyPassword(passwordHash, []byte(password)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
		return false
	}
	return true
}

func ChangePassword(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")

		if !verifyCurrentPassword(c, db, userID, req.CurrentPassword) {
			return
		}

		if err := util.ValidatePassword(req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hash, err := util.HashPassword([]byte(req.NewPassword))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "password hashing failed"})
			return
		}

		if _, err = db.Exec(
			"UPDATE users SET password_hash = $1 WHERE id = $2",
			hash, userID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if _, err = db.Exec(
			"DELETE FROM sessions WHERE user_id = $1",
			userID,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password changed"})
	}
}

func UpdateUsername(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.UpdateUsernameRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")

		if !verifyCurrentPassword(c, db, userID, req.CurrentPassword) {
			return
		}

		if _, err := db.Exec("UPDATE users SET username = $1 WHERE id = $2", req.Username, userID); err != nil {
			if pqErr, ok := errors.AsType[*pgconn.PgError](err); ok && pqErr.Code == pgerrcode.UniqueViolation {
				c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"username": req.Username})
	}
}

func UpdateEmail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.UpdateEmailRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")

		if !verifyCurrentPassword(c, db, userID, req.CurrentPassword) {
			return
		}

		if _, err := db.Exec("UPDATE users SET email = $1 WHERE id = $2", req.Email, userID); err != nil {
			if pqErr, ok := errors.AsType[*pgconn.PgError](err); ok && pqErr.Code == pgerrcode.UniqueViolation {
				c.JSON(http.StatusConflict, gin.H{"error": "email already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"email": req.Email})
	}
}

func DeleteUser(db *sql.DB, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		if strconv.FormatInt(targetID, 10) == c.GetString("userID") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
			return
		}

		if _, err := enforcer.DeleteUser(strconv.FormatInt(targetID, 10)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission error"})
			return
		}

		// FK CASCADE handles access_tokens, sessions, user_invitations
		res, err := db.Exec("DELETE FROM users WHERE id = $1", targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func GetUserPermissions(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		tokenType := c.GetString("tokenType")

		var permissions []string

		if tokenType == auth.TypeSession {
			// Check each known permission via Enforce — handles wildcards (*.*,
			// obj.*) and transitive roles correctly without parsing raw policies.
			for _, perm := range perms.All {
				parts := strings.SplitN(perm, ".", 2)
				if len(parts) != 2 {
					continue
				}
				obj, act := parts[0], parts[1]
				ok, _ := enforcer.Enforce(userID, obj, act, perm)
				if ok {
					permissions = append(permissions, perm)
				}
			}
		} else {
			// access token: scopes set by HandleAccessTokenAuth
			permissions = c.GetStringSlice("tokenScopes")
		}

		if permissions == nil {
			permissions = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"permissions": permissions})
	}
}
