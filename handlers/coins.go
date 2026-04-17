package handlers

import (
	"errors"
	"ledger/auth"
	dbsqlc "ledger/db/sqlc"
	"ledger/mc"
	"ledger/models"
	"ledger/util"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// actorIDs extracts the Ledger user id and access token id (if applicable) from the gin context.
func actorIDs(c *gin.Context) (pgtype.Int8, pgtype.Int8) {
	userIDStr := c.GetString("userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	var actorUserID pgtype.Int8
	if err == nil && userID > 0 {
		actorUserID = pgtype.Int8{Int64: userID, Valid: true}
	}

	var actorTokenID pgtype.Int8
	if c.GetString("tokenType") == auth.TypeAccessToken {
		tokenIDStr := c.GetString("tokenID")
		tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
		if err == nil && tokenID > 0 {
			actorTokenID = pgtype.Int8{Int64: tokenID, Valid: true}
		}
	}

	return actorUserID, actorTokenID
}

func AwardCoins(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		var req models.AwardCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)
		desc := pgtextPtr(req.Description)
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

		if err := q.InsertCoinTransaction(ctx, dbsqlc.InsertCoinTransactionParams{
			PlayerID:     playerID,
			Amount:       req.Amount,
			Source:       dbsqlc.CoinSource(req.Source),
			Description:  desc,
			ActorUserID:  actorUserID,
			ActorTokenID: actorTokenID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := q.UpsertBalance(ctx, dbsqlc.UpsertBalanceParams{
			PlayerID: playerID,
			Balance:  req.Amount,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func SpendCoins(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		var req models.SpendCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)
		desc := pgtextPtr(req.Description)
		ctx := c.Request.Context()

		playerID, err := mc.GetPlayerID(pool, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if playerID == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		q := dbsqlc.New(tx)
		current, err := q.GetLockedBalance(ctx, playerID)
		if errors.Is(err, pgx.ErrNoRows) {
			current = 0
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if current < req.Amount {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient_balance"})
			return
		}

		if err := q.InsertCoinTransaction(ctx, dbsqlc.InsertCoinTransactionParams{
			PlayerID:     playerID,
			Amount:       -req.Amount,
			Source:       dbsqlc.CoinSource(req.Source),
			Description:  desc,
			ActorUserID:  actorUserID,
			ActorTokenID: actorTokenID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := q.UpsertBalance(ctx, dbsqlc.UpsertBalanceParams{
			PlayerID: playerID,
			Balance:  -req.Amount,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func AdjustCoins(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		var req models.AdjustCoinsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		actorUserID, actorTokenID := actorIDs(c)
		desc := pgtextPtr(req.Description)
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

		current, err := q.GetLockedBalance(ctx, playerID)
		if errors.Is(err, pgx.ErrNoRows) {
			current = 0
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if current+req.Amount < 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient_balance"})
			return
		}

		if err := q.InsertCoinTransaction(ctx, dbsqlc.InsertCoinTransactionParams{
			PlayerID:     playerID,
			Amount:       req.Amount,
			Source:       dbsqlc.CoinSource(req.Source),
			Description:  desc,
			ActorUserID:  actorUserID,
			ActorTokenID: actorTokenID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		newBalance, err := q.UpsertBalance(ctx, dbsqlc.UpsertBalanceParams{
			PlayerID: playerID,
			Balance:  req.Amount,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"balance": newBalance})
	}
}

func GetPlayerCoins(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}

		q := dbsqlc.New(pool)
		balance, err := q.GetPlayerBalance(c.Request.Context(), uid)
		if errors.Is(err, pgx.ErrNoRows) {
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

func GetPlayerTransactions(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uuid")
		if !util.ValidateUUID(c, uid) {
			return
		}
		limit, offset := util.ParsePagination(c)

		playerID, err := mc.GetPlayerID(pool, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if playerID == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
			return
		}

		q := dbsqlc.New(pool)
		rows, err := q.ListPlayerTransactions(c.Request.Context(), dbsqlc.ListPlayerTransactionsParams{
			PlayerID: playerID,
			Limit:    int32(limit),
			Offset:   int32(offset),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		transactions := make([]models.CoinTransaction, 0, len(rows))
		for _, r := range rows {
			t := models.CoinTransaction{
				ID:        r.ID,
				PlayerID:  r.PlayerID,
				Amount:    r.Amount,
				Source:    string(r.Source),
				CreatedAt: r.CreatedAt.Time,
			}
			if r.Description.Valid {
				s := r.Description.String
				t.Description = &s
			}
			if r.ActorUserID.Valid {
				n := r.ActorUserID.Int64
				t.ActorUserID = &n
			}
			if r.ActorTokenID.Valid {
				n := r.ActorTokenID.Int64
				t.ActorTokenID = &n
			}
			transactions = append(transactions, t)
		}

		c.JSON(http.StatusOK, transactions)
	}
}

// pgtextPtr converts a *string to pgtype.Text.
func pgtextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
