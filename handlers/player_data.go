package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"ledger/util"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
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

		var newValue any
		if err := json.Unmarshal(body, &newValue); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be valid JSON"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		var rawData []byte
		err = tx.QueryRow(
			`SELECT data FROM minecraft_players WHERE uuid = $1 FOR UPDATE`, uid,
		).Scan(&rawData)
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		var root map[string]any
		if err := json.Unmarshal(rawData, &root); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Navigate/create intermediate nodes, then set the leaf value.
		// jsonb_set(create_missing=true) only creates the leaf key, not
		// intermediate nodes, so we do it in Go inside a FOR UPDATE transaction.
		cur := root
		for _, key := range parts[:len(parts)-1] {
			if m, ok := cur[key].(map[string]any); ok {
				cur = m
			} else {
				m = make(map[string]any)
				cur[key] = m
				cur = m
			}
		}
		cur[parts[len(parts)-1]] = newValue

		newData, err := json.Marshal(root)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if _, err = tx.Exec(
			`UPDATE minecraft_players SET data = $1::jsonb WHERE uuid = $2`,
			string(newData), uid,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
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
