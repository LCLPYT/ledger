package handlers

import (
	"database/sql"
	"errors"
	"ledger/mc"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ListPlayers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, offset := util.ParsePagination(c)
		search := strings.TrimSpace(c.Query("search"))

		rows, err := db.Query(
			`SELECT id, uuid, username, created_at
			 FROM minecraft_players
			 WHERE ($1 = ''
			        OR LOWER(username) LIKE '%' || LOWER($1) || '%'
			        OR LOWER(uuid::text) = LOWER($1))
			 ORDER BY
			   CASE WHEN $1 = '' THEN 0 ELSE 1 END,
			   CASE WHEN LOWER(uuid::text) = LOWER($1) THEN 0 ELSE 1 END,
			   similarity(username, $1) DESC,
			   created_at DESC
			 LIMIT $2 OFFSET $3`,
			search, limit, offset,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		players := make([]models.MinecraftPlayer, 0)
		for rows.Next() {
			var p models.MinecraftPlayer
			if err := rows.Scan(&p.ID, &p.UUID, &p.Username, &p.CreatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			players = append(players, p)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, players)
	}
}

func GetPlayer(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		p, fetchedAt, err := mc.QueryPlayer(db, uid)
		if errors.Is(err, sql.ErrNoRows) {
			// Not in DB: verify via Mojang and create the player if they exist.
			username, err := mc.FetchUsername(uid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch player"})
				return
			}
			if username == "" {
				c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
				return
			}
			p, err = mc.UpsertPlayerWithUsername(db, uid, username)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		} else if fetchedAt == nil || time.Since(*fetchedAt) > mc.UsernameStaleDuration {
			go mc.RefreshUsernameCache(db, p.ID, uid)
		}

		c.JSON(http.StatusOK, p)
	}
}

// LookupPlayerByName resolves a Minecraft username to a UUID.
// It checks the minecraft_players cache first (valid for 1 hour), then falls back to the Mojang API.
func LookupPlayerByName(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}

		// Cache hit: username fetched within the last hour
		var cachedUUID string
		err := db.QueryRow(
			`SELECT uuid FROM minecraft_players
			 WHERE LOWER(username) = LOWER($1)
			   AND username_fetched_at > now() - INTERVAL '1 hour'`,
			name,
		).Scan(&cachedUUID)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"uuid": cachedUUID})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Cache miss: call Mojang API
		mojangUUID, err := mc.FetchUUIDByName(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve player name"})
			return
		}
		if mojangUUID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		// Upsert player with fresh username + timestamp
		p, err := mc.UpsertPlayerWithUsername(db, mojangUUID, name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func DeletePlayer(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}
		result, err := db.Exec("DELETE FROM minecraft_players WHERE uuid = $1", uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
