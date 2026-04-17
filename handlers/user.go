package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"ledger/auth"
	dbsqlc "ledger/db/sqlc"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Login(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		row, err := q.GetUserByIdentifier(c.Request.Context(), req.Identifier)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid username or email"})
			return
		}

		if err := util.VerifyPassword(row.PasswordHash, []byte(req.Password)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password"})
			return
		}

		userIDStr := strconv.FormatInt(row.ID, 10)
		token, err := auth.GenerateSessionToken(userIDStr, pool)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func RefreshSession(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		sessionID := c.GetString("sessionID")

		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}
		sessionIDInt, err := strconv.ParseInt(sessionID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid session id"})
			return
		}

		expiry := time.Now().Add(auth.SessionLifetime)
		q := dbsqlc.New(pool)
		result, err := q.RefreshSession(c.Request.Context(), dbsqlc.RefreshSessionParams{
			ExpiresAt: pgtype.Timestamp{Time: expiry, Valid: true},
			ID:        sessionIDInt,
			UserID:    userIDInt,
		})
		if err != nil || result.RowsAffected() == 0 {
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

func CreateToken(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
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

		token, err := auth.GenerateToken(userID, req.Scopes, req.Expiry, pool, req.Name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func ListTokens(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}

		q := dbsqlc.New(pool)
		rows, err := q.ListTokens(c.Request.Context(), userIDInt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		tokens := make([]models.AccessToken, 0, len(rows))
		for _, r := range rows {
			var scopes []string
			if err := json.Unmarshal(r.Scopes, &scopes); err != nil {
				scopes = []string{}
			}
			tokens = append(tokens, models.AccessToken{
				ID:        r.ID,
				Name:      r.Name,
				CreatedAt: r.CreatedAt.Time,
				ExpiresAt: r.ExpiresAt.Time,
				Revoked:   r.Revoked.Bool,
				Scopes:    scopes,
			})
		}

		c.JSON(http.StatusOK, tokens)
	}
}

func RevokeToken(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}
		tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
			return
		}

		q := dbsqlc.New(pool)
		result, err := q.RevokeToken(c.Request.Context(), dbsqlc.RevokeTokenParams{
			ID:     tokenID,
			UserID: userIDInt,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found or already revoked"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "token revoked"})
	}
}

func ListUsers(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := dbsqlc.New(pool)
		rows, err := q.ListUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		users := make([]models.User, 0, len(rows))
		for _, r := range rows {
			users = append(users, models.User{
				ID:       r.ID,
				Username: r.Username,
				Email:    r.Email,
				Created:  r.CreatedAt.Time,
			})
		}

		c.JSON(http.StatusOK, users)
	}
}

func GetUser(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}

		q := dbsqlc.New(pool)
		row, err := q.GetUserByID(c.Request.Context(), userIDInt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			log.Printf("Failed to query user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, models.User{
			ID:       row.ID,
			Username: row.Username,
			Email:    row.Email,
			Created:  row.CreatedAt.Time,
		})
	}
}

func CreateUser(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		row, err := q.CreateShellUser(c.Request.Context(), dbsqlc.CreateShellUserParams{
			Username: req.Username,
			Email:    req.Email,
		})
		if err != nil {
			if pqErr, ok := errors.AsType[*pgconn.PgError](err); ok && pqErr.Code == pgerrcode.UniqueViolation {
				c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		user := models.User{
			ID:       row.ID,
			Username: row.Username,
			Email:    row.Email,
			Created:  row.CreatedAt.Time,
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

		if err := q.InsertInvitation(c.Request.Context(), dbsqlc.InsertInvitationParams{
			UserID: user.ID,
			Token:  token,
		}); err != nil {
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

func VerifyInvitation(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.VerifyInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		q := dbsqlc.New(pool)
		inv, err := q.GetValidInvitation(c.Request.Context(), req.Token)
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

		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		tq := dbsqlc.New(tx)
		if err := tq.VerifyUser(ctx, dbsqlc.VerifyUserParams{
			PasswordHash: hash,
			ID:           inv.UserID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tq.MarkInvitationUsed(ctx, inv.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password set, account activated"})
	}
}

// verifyCurrentPassword fetches the user's password hash and checks it against
// the provided password. Returns false and writes the appropriate JSON error
// response if the check fails, so the caller can return immediately.
func verifyCurrentPassword(c *gin.Context, q *dbsqlc.Queries, userIDInt int64, password string) bool {
	passwordHash, err := q.GetUserPassword(context.Background(), userIDInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return false
	}
	if err := util.VerifyPassword(passwordHash, []byte(password)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
		return false
	}
	return true
}

func ChangePassword(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}

		q := dbsqlc.New(pool)
		if !verifyCurrentPassword(c, q, userIDInt, req.CurrentPassword) {
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

		if err := q.UpdateUserPassword(c.Request.Context(), dbsqlc.UpdateUserPasswordParams{
			PasswordHash: hash,
			ID:           userIDInt,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := q.DeleteUserSessions(c.Request.Context(), userIDInt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "password changed"})
	}
}

func UpdateUsername(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.UpdateUsernameRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}

		q := dbsqlc.New(pool)
		if !verifyCurrentPassword(c, q, userIDInt, req.CurrentPassword) {
			return
		}

		if err := q.UpdateUsername(c.Request.Context(), dbsqlc.UpdateUsernameParams{
			Username: req.Username,
			ID:       userIDInt,
		}); err != nil {
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

func UpdateEmail(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.UpdateEmailRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		userID := c.GetString("userID")
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
			return
		}

		q := dbsqlc.New(pool)
		if !verifyCurrentPassword(c, q, userIDInt, req.CurrentPassword) {
			return
		}

		if err := q.UpdateEmail(c.Request.Context(), dbsqlc.UpdateEmailParams{
			Email: req.Email,
			ID:    userIDInt,
		}); err != nil {
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

func DeleteUser(pool *pgxpool.Pool, enforcer *casbin.Enforcer) gin.HandlerFunc {
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
		q := dbsqlc.New(pool)
		result, err := q.DeleteUser(c.Request.Context(), targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if result.RowsAffected() == 0 {
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
			permissions = c.GetStringSlice("tokenScopes")
		}

		if permissions == nil {
			permissions = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"permissions": permissions})
	}
}
