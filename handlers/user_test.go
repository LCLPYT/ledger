package handlers_test

import (
	"encoding/json"
	"ledger/middleware"
	"ledger/perms"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ledger/handlers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loginRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/user/login", handlers.Login(testDB, testEnforcer))
	return r
}

func createTokenRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/user/token",
		middleware.AuthRequired(testEnforcer, testDB, "user.create_token"),
		handlers.CreateToken(testDB, testEnforcer))
	return r
}

func getUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/user",
		middleware.AuthRequired(testEnforcer, testDB, "user.read"),
		handlers.GetUser(testDB))
	return r
}

func authedRequest(method, path, bearer, body string) *http.Request {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return req
}

func postLogin(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestLogin_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "secret123")
	mustAddPermission(t, userID, perms.UserRead)

	tests := []struct {
		name       string
		identifier string
	}{
		{"Username", "alice"},
		{"Email", "alice@example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := postLogin(loginRouter(), `{"identifier":"`+test.identifier+`","password":"secret123"}`)

			require.Equal(t, http.StatusOK, w.Code)
			var resp map[string]string
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			assert.NotEmpty(t, resp["token"])
		})
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	cleanDB(t)
	mustCreateUser(t, "carol", "carol@example.com", "correct")

	w := postLogin(loginRouter(), `{"identifier":"carol","password":"wrong"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_UnknownUser(t *testing.T) {
	cleanDB(t)

	w := postLogin(loginRouter(), `{"identifier":"nobody","password":"x"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_MissingFields(t *testing.T) {
	cleanDB(t)

	w := postLogin(loginRouter(), `{}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_TokenStoredInDB(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "dave", "dave@example.com", "pass")
	mustAddPermission(t, userID, perms.UserRead)

	w := postLogin(loginRouter(), `{"identifier":"dave","password":"pass"}`)
	require.Equal(t, http.StatusOK, w.Code)

	var count int
	err := testDB.QueryRow(
		"SELECT COUNT(*) FROM access_tokens WHERE user_id = $1 AND name = 'session' AND revoked = false",
		userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreateToken_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.read", "user.create_token"})

	body := `{"name":"ci","scopes":["user.read"],"expiry":"` + time.Now().Add(24*time.Hour).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", token, body))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
}

func TestCreateToken_Unauthenticated(t *testing.T) {
	cleanDB(t)

	body := `{"name":"x","scopes":["user.read"],"expiry":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", "", body))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateToken_ForbiddenScope(t *testing.T) {
	cleanDB(t)
	// User has "user.read" and "user.create_token" but not "admin.delete"
	userID := mustCreateUser(t, "bob", "bob@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.read", "user.create_token"})

	body := `{"name":"evil","scopes":["admin.delete"],"expiry":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", token, body))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateToken_ExpiryTooFar(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "carol", "carol@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.read", "user.create_token"})

	body := `{"name":"x","scopes":["user.read"],"expiry":"` + time.Now().AddDate(2, 0, 0).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", token, body))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateToken_InvalidScopeFormat(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "dave", "dave@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.read", "user.create_token"})

	body := `{"name":"x","scopes":["notavalidscope"],"expiry":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", token, body))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateToken_StoredInDB(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "eve", "eve@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.read", "user.create_token"})

	body := `{"name":"mytoken","scopes":["user.read"],"expiry":"` + time.Now().Add(24*time.Hour).Format(time.RFC3339) + `"}`
	w := httptest.NewRecorder()
	createTokenRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/user/token", token, body))
	require.Equal(t, http.StatusOK, w.Code)

	var count int
	err := testDB.QueryRow(
		"SELECT COUNT(*) FROM access_tokens WHERE user_id = $1 AND name = 'mytoken' AND revoked = false",
		userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetUser_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "frank", "frank@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	token := mustCreateToken(t, userID, []string{"user.read"})

	w := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/user", token, ""))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "frank", resp["username"])
	assert.Equal(t, "frank@example.com", resp["email"])
}

func TestGetUser_Unauthenticated(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/user", "", ""))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetUser_RevokedToken(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "grace", "grace@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	token := mustCreateToken(t, userID, []string{"user.read"})

	_, err := testDB.Exec("UPDATE access_tokens SET revoked = true WHERE user_id = $1", userID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/user", token, ""))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetUser_InsufficientScope(t *testing.T) {
	cleanDB(t)
	// Token only has "user.create_token" scope, not "user.read"
	userID := mustCreateUser(t, "henry", "henry@example.com", "x")
	mustAddPermission(t, userID, perms.UserRead)
	mustAddPermission(t, userID, perms.UserCreateToken)
	token := mustCreateToken(t, userID, []string{"user.create_token"})

	w := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/user", token, ""))

	assert.Equal(t, http.StatusForbidden, w.Code)
}
