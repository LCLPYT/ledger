package handlers

import (
	"database/sql"
	"errors"
	"ledger/auth"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strconv"
	"time"

	"github.com/casbin/casbin/v2/log"
	"github.com/gin-gonic/gin"
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

		permissions := []string{"user.read", "user.create_token"} // TODO sync user permissions

		token, err := auth.GenerateToken(
			strconv.FormatInt(userID, 10),
			permissions,
			time.Now().Add(7*24*time.Hour),
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}

func CreateToken(c *gin.Context) {
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
	token, err := auth.GenerateToken(userID, req.Permissions, req.Expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func GetUser(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var user models.User

		err := db.QueryRow(
			"SELECT id, username, email, created_at FROM users WHERE user_id = $1",
			userID,
		).Scan(&user.ID, &user.Username, &user.Email, &user.Created)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			}

			log.LogError(err, "Failed to query user")

			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})

			return
		}

		c.JSON(http.StatusOK, user)
	}
}
