package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ledger/handlers"
	"ledger/middleware"
	"ledger/perms"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createUserRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/users",
		middleware.AuthRequired(testEnforcer, testDB, perms.UsersCreate),
		handlers.CreateUser(testDB))
	return r
}

func setPasswordRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/set-password", handlers.SetPassword(testDB))
	return r
}

// strongPassword satisfies all password policy rules.
const strongPassword = "Str0ng!Pass"

// --- CreateUser ---

func TestCreateUser_Success(t *testing.T) {
	cleanDB(t)
	adminID := mustCreateUser(t, "admin", "admin@example.com", "x")
	mustAddPermission(t, adminID, perms.UsersCreate)
	token := mustCreateToken(t, adminID, []string{perms.UsersCreate})

	w := httptest.NewRecorder()
	createUserRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/users", token,
		`{"username":"newuser","email":"new@example.com"}`,
	))

	// 201 regardless of whether the email was delivered (no SMTP in tests)
	require.Equal(t, http.StatusCreated, w.Code)

	// User must exist in DB
	var userID int64
	require.NoError(t, testDB.QueryRow(
		"SELECT id FROM users WHERE username = 'newuser'",
	).Scan(&userID))

	// Invitation row must exist and be unused
	var count int
	require.NoError(t, testDB.QueryRow(
		"SELECT COUNT(*) FROM user_invitations WHERE user_id = $1 AND used_at IS NULL",
		userID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestCreateUser_Conflict(t *testing.T) {
	cleanDB(t)
	mustCreateUser(t, "existing", "existing@example.com", "x")
	adminID := mustCreateUser(t, "admin", "admin@example.com", "x")
	mustAddPermission(t, adminID, perms.UsersCreate)
	token := mustCreateToken(t, adminID, []string{perms.UsersCreate})

	tests := []struct {
		name string
		body string
	}{
		{"duplicate username", `{"username":"existing","email":"other@example.com"}`},
		{"duplicate email", `{"username":"other","email":"existing@example.com"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			createUserRouter().ServeHTTP(w, authedRequest(
				http.MethodPost, "/api/v1/users", token, tc.body,
			))
			assert.Equal(t, http.StatusConflict, w.Code)
		})
	}
}

func TestCreateUser_Unauthorized(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	createUserRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/users", "",
		`{"username":"newuser","email":"new@example.com"}`,
	))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateUser_Forbidden(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "user", "user@example.com", "x")
	mustAddPermission(t, userID, perms.UsersRead) // read but not create
	token := mustCreateToken(t, userID, []string{perms.UsersRead})

	w := httptest.NewRecorder()
	createUserRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/users", token,
		`{"username":"newuser","email":"new@example.com"}`,
	))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateUser_MissingFields(t *testing.T) {
	cleanDB(t)
	adminID := mustCreateUser(t, "admin", "admin@example.com", "x")
	mustAddPermission(t, adminID, perms.UsersCreate)
	token := mustCreateToken(t, adminID, []string{perms.UsersCreate})

	tests := []struct {
		name string
		body string
	}{
		{"empty body", `{}`},
		{"missing email", `{"username":"newuser"}`},
		{"missing username", `{"email":"new@example.com"}`},
		{"invalid email format", `{"username":"newuser","email":"notanemail"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			createUserRouter().ServeHTTP(w, authedRequest(
				http.MethodPost, "/api/v1/users", token, tc.body,
			))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// --- SetPassword ---

func TestSetPassword_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "validtok", time.Now().Add(24*time.Hour))

	w := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"validtok","password":"`+strongPassword+`"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)

	// verified_at must be set
	var verifiedAt *time.Time
	require.NoError(t, testDB.QueryRow(
		"SELECT verified_at FROM users WHERE id = $1", userID,
	).Scan(&verifiedAt))
	assert.NotNil(t, verifiedAt)

	// Invitation must be marked used
	var usedAt *time.Time
	require.NoError(t, testDB.QueryRow(
		"SELECT used_at FROM user_invitations WHERE token = 'validtok'",
	).Scan(&usedAt))
	assert.NotNil(t, usedAt)
}

func TestSetPassword_AllowsLogin(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "tok", time.Now().Add(24*time.Hour))

	w := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"tok","password":"`+strongPassword+`"}`,
	))
	require.Equal(t, http.StatusOK, w.Code)

	// User must now be able to log in
	w2 := postLogin(loginRouter(), `{"identifier":"newuser","password":"`+strongPassword+`"}`)
	require.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&resp))
	assert.NotEmpty(t, resp["token"])
}

func TestSetPassword_InvalidToken(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"doesnotexist","password":"`+strongPassword+`"}`,
	))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetPassword_ExpiredToken(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	// Use DB-side now() to avoid host↔container clock conversion issues.
	_, err := testDB.Exec(
		"INSERT INTO user_invitations (user_id, token, expires_at) VALUES ($1, $2, now() - interval '1 minute')",
		userID, "expiredtok",
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"expiredtok","password":"`+strongPassword+`"}`,
	))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetPassword_AlreadyUsed(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "usedtok", time.Now().Add(24*time.Hour))

	// First use — succeeds
	w := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"usedtok","password":"`+strongPassword+`"}`,
	))
	require.Equal(t, http.StatusOK, w.Code)

	// Second use — must fail
	w2 := httptest.NewRecorder()
	setPasswordRouter().ServeHTTP(w2, authedRequest(
		http.MethodPost, "/api/v1/auth/set-password", "",
		`{"token":"usedtok","password":"Another!Strong1"}`,
	))
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestSetPassword_WeakPassword(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "tok", time.Now().Add(24*time.Hour))

	tests := []struct {
		name     string
		password string
	}{
		{"too short", "Ab1!"},
		{"no uppercase", "str0ng!pass"},
		{"no lowercase", "STR0NG!PASS"},
		{"no digit", "Strong!Pass"},
		{"no special", "Str0ngPass"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			setPasswordRouter().ServeHTTP(w, authedRequest(
				http.MethodPost, "/api/v1/auth/set-password", "",
				`{"token":"tok","password":"`+tc.password+`"}`,
			))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}

	// Token must still be unused — no weak-password attempt should consume it
	var usedAt *time.Time
	require.NoError(t, testDB.QueryRow(
		"SELECT used_at FROM user_invitations WHERE token = 'tok'",
	).Scan(&usedAt))
	assert.Nil(t, usedAt)
}
