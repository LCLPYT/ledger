package handlers

import (
	"database/sql"
	"errors"
	"ledger/auth"
	"ledger/mc"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// validateUUID returns false and writes a 400 response if the string is not a valid UUID.
func validateUUID(c *gin.Context, raw string) bool {
	if _, err := uuid.Parse(raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return false
	}
	return true
}

// actorIDs extracts the Ledger user id and access token id (if applicable) from the gin context.
func actorIDs(c *gin.Context) (*int64, *int64) {
	userIDStr := c.GetString("userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	var actorUserID *int64
	if err == nil && userID > 0 {
		actorUserID = &userID
	}

	var actorTokenID *int64
	if c.GetString("tokenType") == auth.TypeAccessToken {
		tokenIDStr := c.GetString("tokenID")
		tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
		if err == nil && tokenID > 0 {
			actorTokenID = &tokenID
		}
	}

	return actorUserID, actorTokenID
}

// insertTransaction records a coin transaction row inside an open transaction.
func insertTransaction(tx *sql.Tx, playerID, amount int64, source string, description *string, actorUserID, actorTokenID *int64) error {
	_, err := tx.Exec(
		`INSERT INTO coin_transactions (player_id, amount, source, description, actor_user_id, actor_token_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		playerID, amount, source, description, actorUserID, actorTokenID,
	)
	return err
}

// lockedBalance reads the current balance for a player with a FOR UPDATE lock.
// Returns 0 (not an error) when no balance row exists yet.
func lockedBalance(tx *sql.Tx, playerID int64) (int64, error) {
	var balance int64
	err := tx.QueryRow(
		"SELECT balance FROM coin_balances WHERE player_id = $1 FOR UPDATE",
		playerID,
	).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return balance, err
}

// applyBalanceDelta upserts the coin_balances row, adding delta to the current balance.
// delta may be positive or negative.
func applyBalanceDelta(tx *sql.Tx, playerID, delta int64) (int64, error) {
	var newBalance int64
	err := tx.QueryRow(
		`INSERT INTO coin_balances (player_id, balance, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (player_id) DO UPDATE
		   SET balance = coin_balances.balance + $2, updated_at = now()
		 RETURNING balance`,
		playerID, delta,
	).Scan(&newBalance)
	return newBalance, err
}

func AwardCoins(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
			return
		}

		var req models.AwardCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		playerID, err := mc.UpsertPlayer(db, tx, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := insertTransaction(tx, playerID, req.Amount, req.Source, req.Description, actorUserID, actorTokenID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := applyBalanceDelta(tx, playerID, req.Amount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func SpendCoins(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
			return
		}

		var req models.SpendCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)

		playerID, err := mc.GetPlayerID(db, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if playerID == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		current, err := lockedBalance(tx, playerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if current < req.Amount {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient_balance"})
			return
		}

		if err := insertTransaction(tx, playerID, -req.Amount, req.Source, req.Description, actorUserID, actorTokenID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := applyBalanceDelta(tx, playerID, -req.Amount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func AdjustCoins(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
			return
		}

		var req models.AdjustCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		playerID, err := mc.UpsertPlayer(db, tx, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		current, err := lockedBalance(tx, playerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if current+req.Amount < 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient_balance"})
			return
		}

		if err := insertTransaction(tx, playerID, req.Amount, req.Source, req.Description, actorUserID, actorTokenID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := applyBalanceDelta(tx, playerID, req.Amount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func DeletePlayer(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
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

func GetPlayerCoins(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
			return
		}

		var balance int64
		err := db.QueryRow(
			`SELECT cb.balance FROM coin_balances cb
			 JOIN minecraft_players mp ON mp.id = cb.player_id
			 WHERE mp.uuid = $1`,
			uid,
		).Scan(&balance)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": balance})
	}
}

func GetPlayerTransactions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !validateUUID(c, uid) {
			return
		}
		limit, offset := util.ParsePagination(c)

		playerID, err := mc.GetPlayerID(db, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if playerID == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		rows, err := db.Query(
			`SELECT id, player_id, amount, source, description, created_at, actor_user_id, actor_token_id
			 FROM coin_transactions WHERE player_id = $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			playerID, limit, offset,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer func() { _ = rows.Close() }()

		transactions := make([]models.CoinTransaction, 0)
		for rows.Next() {
			var t models.CoinTransaction
			if err := rows.Scan(
				&t.ID, &t.PlayerID, &t.Amount, &t.Source, &t.Description,
				&t.CreatedAt, &t.ActorUserID, &t.ActorTokenID,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			transactions = append(transactions, t)
		}
		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, transactions)
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

		c.JSON(http.StatusOK, gin.H{"uuid": mojangUUID})
	}
}

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
