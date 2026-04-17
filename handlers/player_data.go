package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	dbsqlc "ledger/db/sqlc"
	"ledger/mc"
	"ledger/util"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func GetPlayerData(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		rawPath := strings.TrimPrefix(c.Param("path"), "/")
		parts := splitPath(rawPath)

		q := dbsqlc.New(pool)
		ctx := c.Request.Context()

		if len(parts) == 0 {
			raw, err := q.GetPlayerData(ctx, uid)
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
			return
		}

		result, err := q.GetPlayerDataAtPath(ctx, dbsqlc.GetPlayerDataAtPathParams{
			Column1: parts,
			Uuid:    uid,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if result == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "path not found"})
			return
		}
		raw, err := json.Marshal(result)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", raw)
	}
}

func SetPlayerData(pool *pgxpool.Pool) gin.HandlerFunc {
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

		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		q := dbsqlc.New(tx)
		playerID, err := mc.UpsertPlayer(pool, q, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		rawData, err := q.GetPlayerDataForUpdate(ctx, playerID)
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

		if err := q.UpdatePlayerData(ctx, dbsqlc.UpdatePlayerDataParams{
			Column1: newData,
			ID:      playerID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func DeletePlayerData(pool *pgxpool.Pool) gin.HandlerFunc {
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

		q := dbsqlc.New(pool)
		result, err := q.DeletePlayerDataAtPath(c.Request.Context(), dbsqlc.DeletePlayerDataAtPathParams{
			Column1: parts,
			Uuid:    uid,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
