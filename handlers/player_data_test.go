package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ledger/handlers"
	"ledger/middleware"
	"ledger/perms"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Router factories ──────────────────────────────────────────────────────────

func playerDataRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/minecraft/players/:uuid/data",
		middleware.AuthRequired(testEnforcer, testDB, perms.PlayerDataRead),
		handlers.GetPlayerData(testDB))
	r.GET("/api/v1/minecraft/players/:uuid/data/*path",
		middleware.AuthRequired(testEnforcer, testDB, perms.PlayerDataRead),
		handlers.GetPlayerData(testDB))
	r.PUT("/api/v1/minecraft/players/:uuid/data/*path",
		middleware.AuthRequired(testEnforcer, testDB, perms.PlayerDataWrite),
		handlers.SetPlayerData(testDB))
	r.DELETE("/api/v1/minecraft/players/:uuid/data/*path",
		middleware.AuthRequired(testEnforcer, testDB, perms.PlayerDataWrite),
		handlers.DeletePlayerData(testDB))
	return r
}

// mustSetPlayerData writes directly to the DB for test setup.
func mustSetPlayerData(t *testing.T, uuid string, dataJSON string) {
	t.Helper()
	_, err := testDB.Exec(
		`UPDATE minecraft_players SET data = $1::jsonb WHERE uuid = $2`,
		dataJSON, uuid,
	)
	require.NoError(t, err)
}

func getPlayerData(t *testing.T, uuid string) json.RawMessage {
	t.Helper()
	var raw []byte
	err := testDB.QueryRow(
		`SELECT data FROM minecraft_players WHERE uuid = $1`, uuid,
	).Scan(&raw)
	require.NoError(t, err)
	return raw
}

const testPlayerUUID = "069a79f4-44e9-4726-a5be-fca90e38aaf5"

func setupPlayerDataTest(t *testing.T) (r *gin.Engine, readToken, writeToken string) {
	t.Helper()
	cleanDB(t)
	mustCreatePlayer(t, testPlayerUUID)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.PlayerDataRead)
	mustAddPermission(t, userID, perms.PlayerDataWrite)
	readToken = mustCreateToken(t, userID, []string{perms.PlayerDataRead})
	writeToken = mustCreateToken(t, userID, []string{perms.PlayerDataWrite})
	r = playerDataRouter()
	return
}

// ── GetPlayerData (full blob) ─────────────────────────────────────────────────

func TestGetPlayerData_FullBlob_Empty(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{}`, w.Body.String())
}

func TestGetPlayerData_FullBlob_WithData(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"lang":"en","score":42}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"lang":"en","score":42}`, w.Body.String())
}

func TestGetPlayerData_FullBlob_PlayerNotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.PlayerDataRead)
	token := mustCreateToken(t, userID, []string{perms.PlayerDataRead})

	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data", token, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPlayerData_FullBlob_Unauthorized(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data", "", "",
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── GetPlayerData (at path) ───────────────────────────────────────────────────

func TestGetPlayerData_AtPath_SimpleKey(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"lang":"en","score":42}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `"en"`, w.Body.String())
}

func TestGetPlayerData_AtPath_Nested(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"game":{"soccer":{"goals":7}}}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/game/soccer", readToken, "",
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"goals":7}`, w.Body.String())
}

func TestGetPlayerData_AtPath_NotFound(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/nonexistent", readToken, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPlayerData_AtPath_PlayerNotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.PlayerDataRead)
	token := mustCreateToken(t, userID, []string{perms.PlayerDataRead})

	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodGet, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", token, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── SetPlayerData ─────────────────────────────────────────────────────────────

func TestSetPlayerData_SimpleKey(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", writeToken, `"en"`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"ok":true}`, w.Body.String())
	assert.JSONEq(t, `{"lang":"en"}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_NestedKeyCreatesIntermediateNodes(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/game/soccer/goals", writeToken, `7`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"game":{"soccer":{"goals":7}}}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_ReplacesSubtree(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"game":{"soccer":{"goals":7,"color":"red"}}}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/game/soccer", writeToken, `{"goals":0}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	// color must be gone — subtree replaced, not merged
	assert.JSONEq(t, `{"game":{"soccer":{"goals":0}}}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_PreservesOtherKeys(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"lang":"en","score":10}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/score", writeToken, `99`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"lang":"en","score":99}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_ObjectValue(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/prefs", writeToken, `{"theme":"dark","volume":80}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"prefs":{"theme":"dark","volume":80}}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_AutoCreatesPlayer(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.PlayerDataWrite)
	token := mustCreateToken(t, userID, []string{perms.PlayerDataWrite})

	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", token, `"en"`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"lang":"en"}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestSetPlayerData_InvalidJSON(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", writeToken, `not-json`,
	))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetPlayerData_EmptyBody(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", writeToken, ``,
	))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetPlayerData_Unauthorized(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", "", `"en"`,
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSetPlayerData_WrongScope(t *testing.T) {
	r, readToken, _ := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", readToken, `"en"`,
	))
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── DeletePlayerData ──────────────────────────────────────────────────────────

func TestDeletePlayerData_SimpleKey(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"lang":"en","score":10}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", writeToken, "",
	))

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.JSONEq(t, `{"score":10}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestDeletePlayerData_NestedKey(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"game":{"soccer":{"goals":7,"color":"red"}}}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/game/soccer/color", writeToken, "",
	))

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.JSONEq(t, `{"game":{"soccer":{"goals":7}}}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestDeletePlayerData_Subtree(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)
	mustSetPlayerData(t, testPlayerUUID, `{"lang":"en","game":{"soccer":{"goals":7}}}`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/game", writeToken, "",
	))

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.JSONEq(t, `{"lang":"en"}`, string(getPlayerData(t, testPlayerUUID)))
}

func TestDeletePlayerData_NonexistentKey_StillOK(t *testing.T) {
	r, _, writeToken := setupPlayerDataTest(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/nonexistent", writeToken, "",
	))
	// #- on missing key is a no-op in Postgres — player exists so we return 204
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeletePlayerData_PlayerNotFound(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.PlayerDataWrite)
	token := mustCreateToken(t, userID, []string{perms.PlayerDataWrite})

	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", token, "",
	))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeletePlayerData_Unauthorized(t *testing.T) {
	cleanDB(t)
	w := httptest.NewRecorder()
	playerDataRouter().ServeHTTP(w, authedRequest(
		http.MethodDelete, "/api/v1/minecraft/players/"+testPlayerUUID+"/data/lang", "", "",
	))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
