package handlers

import (
	"errors"
	dbsqlc "ledger/db/sqlc"
	"ledger/mc"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ListPlayers(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, offset := util.ParsePagination(c)
		search := strings.TrimSpace(c.Query("search"))

		q := dbsqlc.New(pool)
		rows, err := q.ListPlayers(c.Request.Context(), dbsqlc.ListPlayersParams{
			Column1: search,
			Limit:   int32(limit),
			Offset:  int32(offset),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		players := make([]models.MinecraftPlayer, 0, len(rows))
		for _, r := range rows {
			p := models.MinecraftPlayer{
				ID:        r.ID,
				UUID:      r.Uuid,
				CreatedAt: r.CreatedAt.Time,
			}
			if r.Username.Valid {
				s := r.Username.String
				p.Username = &s
			}
			players = append(players, p)
		}

		c.JSON(http.StatusOK, players)
	}
}

func GetPlayer(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		p, fetchedAt, err := mc.QueryPlayer(pool, uid)
		if errors.Is(err, pgx.ErrNoRows) {
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
			p, err = mc.UpsertPlayerWithUsername(pool, uid, username)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		} else if fetchedAt == nil || time.Since(*fetchedAt) > mc.UsernameStaleDuration {
			go mc.RefreshUsernameCache(pool, p.ID, uid)
		}

		c.JSON(http.StatusOK, p)
	}
}

// LookupPlayerByName resolves a Minecraft username to a UUID.
// It checks the minecraft_players cache first (valid for 1 hour), then falls back to the Mojang API.
func LookupPlayerByName(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Query("name"))
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}

		q := dbsqlc.New(pool)
		cachedUUID, err := q.GetPlayerUUIDByName(c.Request.Context(), name)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"uuid": cachedUUID})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
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
		p, err := mc.UpsertPlayerWithUsername(pool, mojangUUID, name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func DeletePlayer(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		q := dbsqlc.New(pool)
		result, err := q.DeletePlayerByUUID(c.Request.Context(), uid)
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
