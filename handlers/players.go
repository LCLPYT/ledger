package handlers

import (
	"database/sql"
	"errors"
	"ledger/mc"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ListPlayers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, offset := util.ParsePagination(c)

		rows, err := db.Query(
			`SELECT mp.id, mp.uuid, mp.username, mp.created_at, COALESCE(cb.balance, 0)
			 FROM minecraft_players mp
			 LEFT JOIN coin_balances cb ON cb.player_id = mp.id
			 ORDER BY mp.created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		players := make([]models.MinecraftPlayer, 0)
		for rows.Next() {
			var p models.MinecraftPlayer
			if err := rows.Scan(&p.ID, &p.UUID, &p.Username, &p.CreatedAt, &p.Balance); err != nil {
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

		var p models.MinecraftPlayer
		err := db.QueryRow(
			`SELECT id, uuid, username, created_at,
			FROM minecraft_players WHERE uuid = $1`,
			uid,
		).Scan(&p.ID, &p.UUID, &p.Username, &p.CreatedAt)

		// TODO handle error and return player json
		// TODO if uuid doesn't exist in the database, fetch from mojang
		// TODO if username is stale, trigger fetch in the background

		if err != nil { /*todo*/
		}
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
		_, err = db.Exec(
			`INSERT INTO minecraft_players (uuid, username, username_fetched_at)
			 VALUES ($1, $2, now())
			 ON CONFLICT (uuid) DO UPDATE
			   SET username = EXCLUDED.username,
			       username_fetched_at = now()`,
			mojangUUID, name,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// TODO should return whole player json
		c.JSON(http.StatusOK, gin.H{"uuid": mojangUUID})
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
