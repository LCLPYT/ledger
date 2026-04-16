package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"ledger/util"
)

// splitPath converts "game/soccer/foo" → ["game","soccer","foo"].
// Returns nil if path is empty after trimming.
func splitPath(raw string) []string {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}

func GetPlayerData(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		rawPath := strings.TrimPrefix(c.Param("path"), "/")
		parts := splitPath(rawPath)

		var raw []byte
		var err error

		if len(parts) == 0 {
			err = db.QueryRow(
				`SELECT data FROM minecraft_players WHERE uuid = $1`,
				uid,
			).Scan(&raw)
		} else {
			err = db.QueryRow(
				`SELECT data #> $1 FROM minecraft_players WHERE uuid = $2`,
				pq.Array(parts), uid,
			).Scan(&raw)
		}

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if raw == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "path not found"})
			return
		}

		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	}
}

func SetPlayerData(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		rawPath := strings.TrimPrefix(c.Param("path"), "/")
		parts := splitPath(rawPath)
		if len(parts) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil || len(body) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "request body is required"})
			return
		}
		if !json.Valid(body) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be valid JSON"})
			return
		}

		result, err := db.Exec(
			`UPDATE minecraft_players
			 SET data = jsonb_set(data, $1, $2::jsonb, true)
			 WHERE uuid = $3`,
			pq.Array(parts), string(body), uid,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func DeletePlayerData(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		rawPath := strings.TrimPrefix(c.Param("path"), "/")
		parts := splitPath(rawPath)
		if len(parts) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
			return
		}

		result, err := db.Exec(
			`UPDATE minecraft_players
			 SET data = data #- $1
			 WHERE uuid = $2`,
			pq.Array(parts), uid,
		)
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
