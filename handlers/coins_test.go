package handlers_test

import (
	"encoding/json"
	"ledger/mc"
	"net/http"
	"net/http/httptest"
	"testing"

	"ledger/handlers"
	"ledger/middleware"
	"ledger/models"
	"ledger/perms"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Router factories ──────────────────────────────────────────────────────────

func awardCoinsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/minecraft/players/:uuid/coins/award",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsWrite),
		handlers.AwardCoins(testDB))
	return r
}

func spendCoinsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/minecraft/players/:uuid/coins/spend",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsWrite),
		handlers.SpendCoins(testDB))
	return r
}

func adjustCoinsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/minecraft/players/:uuid/coins/adjust",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsWrite),
		handlers.AdjustCoins(testDB))
	return r
}

func getPlayerCoinsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/minecraft/players/:uuid/coins",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsRead),
		handlers.GetPlayerCoins(testDB))
	return r
}

func getPlayerTransactionsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/minecraft/players/:uuid/coins/transactions",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsRead),
		handlers.GetPlayerTransactions(testDB))
	return r
}

func listPlayersRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/minecraft/players",
		middleware.AuthRequired(testEnforcer, testDB, perms.CoinsRead),
		handlers.ListPlayers(testDB))
	return r
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func mustCreatePlayer(t *testing.T, uuid string) int64 {
	t.Helper()
	tx, err := testDB.Begin()
	require.NoError(t, err)
	id, err := mc.UpsertPlayer(testDB, tx, uuid)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	return id
}

// ── UpsertPlayer tests ────────────────────────────────────────────────────────

func TestUpsertPlayer_NewPlayer(t *testing.T) {
	cleanDB(t)
	tx, err := testDB.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	id, err := mc.UpsertPlayer(testDB, tx, "069a79f4-44e9-4726-a5be-fca90e38aaf5")
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, testDB.QueryRow(
		"SELECT COUNT(*) FROM minecraft_players WHERE id = $1", id,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertPlayer_ExistingPlayerReturnsSameID(t *testing.T) {
	cleanDB(t)
	const playerUUID = "069a79f4-44e9-4726-a5be-fca90e38aaf5"

	tx1, err := testDB.Begin()
	require.NoError(t, err)
	id1, err := mc.UpsertPlayer(testDB, tx1, playerUUID)
	require.NoError(t, err)
	require.NoError(t, tx1.Commit())

	tx2, err := testDB.Begin()
	require.NoError(t, err)
	id2, err := mc.UpsertPlayer(testDB, tx2, playerUUID)
	require.NoError(t, err)
	require.NoError(t, tx2.Commit())

	assert.Equal(t, id1, id2)
}

func mustSetBalance(t *testing.T, playerID, balance int64) {
	t.Helper()
	_, err := testDB.Exec(
		`INSERT INTO coin_balances (player_id, balance) VALUES ($1, $2)
		 ON CONFLICT (player_id) DO UPDATE SET balance = $2`,
		playerID, balance,
	)
	require.NoError(t, err)
}

func getBalance(t *testing.T, playerID int64) int64 {
	t.Helper()
	var b int64
	err := testDB.QueryRow(
		"SELECT COALESCE((SELECT balance FROM coin_balances WHERE player_id = $1), 0)",
		playerID,
	).Scan(&b)
	require.NoError(t, err)
	return b
}

func countTransactions(t *testing.T, playerID int64) int {
	t.Helper()
	var n int
	err := testDB.QueryRow(
		"SELECT COUNT(*) FROM coin_transactions WHERE player_id = $1", playerID,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

// ── AwardCoins ────────────────────────────────────────────────────────────────

func TestAwardCoins_NewPlayer(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/award", token,
		`{"amount":100,"source":"minigame"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(100), resp["balance"])

	playerID, err := mc.GetPlayerID(testDB, uuid)
	require.NoError(t, err)
	assert.NotZero(t, playerID)
	assert.Equal(t, int64(100), getBalance(t, playerID))
	assert.Equal(t, 1, countTransactions(t, playerID))
}

func TestAwardCoins_AccumulatesBalance(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 50)

	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/award", token,
		`{"amount":100,"source":"minigame"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(150), resp["balance"])
	assert.Equal(t, int64(150), getBalance(t, playerID))
}

func TestAwardCoins_NoToken(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/award", "",
		`{"amount":100,"source":"minigame"}`,
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAwardCoins_WrongScope(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/award", token,
		`{"amount":100,"source":"minigame"}`,
	))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAwardCoins_InvalidAmount(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/award", token,
		`{"amount":0,"source":"minigame"}`,
	))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAwardCoins_InvalidSource(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	w := httptest.NewRecorder()
	awardCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/award", token,
		`{"amount":100,"source":"invalid"}`,
	))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── SpendCoins ────────────────────────────────────────────────────────────────

func TestSpendCoins_HappyPath(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 100)

	w := httptest.NewRecorder()
	spendCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/spend", token,
		`{"amount":25,"source":"purchase"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(75), resp["balance"])
	assert.Equal(t, int64(75), getBalance(t, playerID))
	assert.Equal(t, 1, countTransactions(t, playerID))
}

func TestSpendCoins_InsufficientBalance(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 10)

	w := httptest.NewRecorder()
	spendCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/spend", token,
		`{"amount":20,"source":"purchase"}`,
	))

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "insufficient_balance", resp["error"])
	assert.Equal(t, int64(10), getBalance(t, playerID))
	assert.Equal(t, 0, countTransactions(t, playerID))
}

func TestSpendCoins_PlayerNotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	w := httptest.NewRecorder()
	spendCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/spend", token,
		`{"amount":10,"source":"purchase"}`,
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSpendCoins_NoToken(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	spendCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/spend", "",
		`{"amount":10,"source":"purchase"}`,
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSpendCoins_InvalidAmount(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	w := httptest.NewRecorder()
	spendCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/spend", token,
		`{"amount":0,"source":"purchase"}`,
	))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── AdjustCoins ───────────────────────────────────────────────────────────────

func TestAdjustCoins_PositiveCreatesPlayer(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	w := httptest.NewRecorder()
	adjustCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/adjust", token,
		`{"amount":50,"source":"admin"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(50), resp["balance"])

	playerID, err := mc.GetPlayerID(testDB, uuid)
	require.NoError(t, err)
	assert.NotZero(t, playerID)
	assert.Equal(t, int64(50), getBalance(t, playerID))
}

func TestAdjustCoins_NegativeDeducts(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 100)

	w := httptest.NewRecorder()
	adjustCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/adjust", token,
		`{"amount":-30,"source":"admin"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(70), resp["balance"])
	assert.Equal(t, int64(70), getBalance(t, playerID))
}

func TestAdjustCoins_WouldGoNegative(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	token := mustCreateToken(t, userID, []string{perms.CoinsWrite})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 10)

	w := httptest.NewRecorder()
	adjustCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/adjust", token,
		`{"amount":-20,"source":"admin"}`,
	))

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, int64(10), getBalance(t, playerID))
}

func TestAdjustCoins_NoToken(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	adjustCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/adjust", "",
		`{"amount":50,"source":"admin"}`,
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── GetPlayerCoins ────────────────────────────────────────────────────────────

func TestGetPlayerCoins_Found(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	playerID := mustCreatePlayer(t, uuid)
	mustSetBalance(t, playerID, 200)

	w := httptest.NewRecorder()
	getPlayerCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+uuid+"/coins", token, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, float64(200), resp["balance"])
}

func TestGetPlayerCoins_NotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	w := httptest.NewRecorder()
	getPlayerCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins", token, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPlayerCoins_Unauthorized(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	getPlayerCoinsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins", "", "",
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── GetPlayerTransactions ─────────────────────────────────────────────────────

func TestGetPlayerTransactions_ReturnsList(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	mustAddPermission(t, userID, perms.CoinsRead)
	writeToken := mustCreateToken(t, userID, []string{perms.CoinsWrite})
	readToken := mustCreateToken(t, userID, []string{perms.CoinsRead})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	r := awardCoinsRouter()
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedRequest(
			http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/award", writeToken,
			`{"amount":10,"source":"minigame"}`,
		))
		require.Equal(t, http.StatusOK, w.Code)
	}

	w := httptest.NewRecorder()
	getPlayerTransactionsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+uuid+"/coins/transactions", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	var txns []models.CoinTransaction
	require.NoError(t, json.NewDecoder(w.Body).Decode(&txns))
	assert.Len(t, txns, 2)
	// Descending order: most recent first
	assert.GreaterOrEqual(t, txns[0].CreatedAt.UnixNano(), txns[1].CreatedAt.UnixNano())
}

func TestGetPlayerTransactions_PlayerNotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	w := httptest.NewRecorder()
	getPlayerTransactionsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/069a79f4-44e9-4726-a5be-fca90e38aaf5/coins/transactions", token, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPlayerTransactions_Pagination(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsWrite)
	mustAddPermission(t, userID, perms.CoinsRead)
	writeToken := mustCreateToken(t, userID, []string{perms.CoinsWrite})
	readToken := mustCreateToken(t, userID, []string{perms.CoinsRead})

	uuid := "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	r := awardCoinsRouter()
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, authedRequest(
			http.MethodPost, "/api/v1/minecraft/players/"+uuid+"/coins/award", writeToken,
			`{"amount":10,"source":"minigame"}`,
		))
		require.Equal(t, http.StatusOK, w.Code)
	}

	w := httptest.NewRecorder()
	getPlayerTransactionsRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+uuid+"/coins/transactions?limit=2&offset=0", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	var txns []models.CoinTransaction
	require.NoError(t, json.NewDecoder(w.Body).Decode(&txns))
	assert.Len(t, txns, 2)
}

// ── ListPlayers ───────────────────────────────────────────────────────────────

func TestListPlayers_ReturnsList(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	uuids := []string{
		"069a79f4-44e9-4726-a5be-fca90e38aaf5",
		"853c80ef-3a6e-4d29-b5a9-a7dc12a4c0ca",
	}
	balances := []int64{100, 250}
	for i, uuid := range uuids {
		pid := mustCreatePlayer(t, uuid)
		mustSetBalance(t, pid, balances[i])
	}

	w := httptest.NewRecorder()
	listPlayersRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/minecraft/players", token, ""))

	require.Equal(t, http.StatusOK, w.Code)
	var players []models.MinecraftPlayer
	require.NoError(t, json.NewDecoder(w.Body).Decode(&players))
	assert.Len(t, players, 2)

	// Verify both UUIDs and their balances appear
	found := map[string]int64{}
	for _, p := range players {
		found[p.UUID] = p.Balance
	}
	for i, uuid := range uuids {
		assert.Equal(t, balances[i], found[uuid], "balance mismatch for %s", uuid)
	}
}

func TestListPlayers_Empty(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.CoinsRead)
	token := mustCreateToken(t, userID, []string{perms.CoinsRead})

	w := httptest.NewRecorder()
	listPlayersRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/minecraft/players", token, ""))

	require.Equal(t, http.StatusOK, w.Code)
	var players []models.MinecraftPlayer
	require.NoError(t, json.NewDecoder(w.Body).Decode(&players))
	assert.Empty(t, players)
}

func TestListPlayers_Unauthorized(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	listPlayersRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/minecraft/players", "", ""))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
