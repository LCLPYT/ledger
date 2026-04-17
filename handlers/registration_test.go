package handlers_test

import (
	"context"
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
		middleware.AuthRequired(testEnforcer, testPool, perms.UsersCreate),
		handlers.CreateUser(testPool, testEnforcer))
	return r
}

func verifyInvitationRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/auth/verify-invitation", handlers.VerifyInvitation(testPool))
	return r
}

func changePasswordRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/user/password", middleware.SessionRequired(testPool), handlers.ChangePassword(testPool))
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
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT id FROM users WHERE username = 'newuser'",
	).Scan(&userID))

	// Invitation row must exist and be unused
	var count int
	require.NoError(t, testPool.QueryRow(context.Background(),
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

// --- VerifyInvitation ---

func TestVerifyInvitation_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "validtok", time.Now().Add(24*time.Hour))

	w := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
		`{"token":"validtok","password":"`+strongPassword+`"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)

	// verified_at must be set
	var verifiedAt *time.Time
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT verified_at FROM users WHERE id = $1", userID,
	).Scan(&verifiedAt))
	assert.NotNil(t, verifiedAt)

	// Invitation must be marked used
	var usedAt *time.Time
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT used_at FROM user_invitations WHERE token = 'validtok'",
	).Scan(&usedAt))
	assert.NotNil(t, usedAt)
}

func TestVerifyInvitation_AllowsLogin(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "tok", time.Now().Add(24*time.Hour))

	w := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
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

func TestVerifyInvitation_InvalidToken(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
		`{"token":"doesnotexist","password":"`+strongPassword+`"}`,
	))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVerifyInvitation_ExpiredToken(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	// Use DB-side now() to avoid host↔container clock conversion issues.
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO user_invitations (user_id, token, expires_at) VALUES ($1, $2, now() - interval '1 minute')",
		userID, "expiredtok",
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
		`{"token":"expiredtok","password":"`+strongPassword+`"}`,
	))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVerifyInvitation_AlreadyUsed(t *testing.T) {
	cleanDB(t)
	userID := mustCreateShellUser(t, "newuser", "new@example.com")
	mustCreateInvitation(t, userID, "usedtok", time.Now().Add(24*time.Hour))

	// First use — succeeds
	w := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
		`{"token":"usedtok","password":"`+strongPassword+`"}`,
	))
	require.Equal(t, http.StatusOK, w.Code)

	// Second use — must fail
	w2 := httptest.NewRecorder()
	verifyInvitationRouter().ServeHTTP(w2, authedRequest(
		http.MethodPost, "/api/v1/auth/verify-invitation", "",
		`{"token":"usedtok","password":"Another!Strong1"}`,
	))
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestVerifyInvitation_WeakPassword(t *testing.T) {
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
			verifyInvitationRouter().ServeHTTP(w, authedRequest(
				http.MethodPost, "/api/v1/auth/verify-invitation", "",
				`{"token":"tok","password":"`+tc.password+`"}`,
			))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}

	// Token must still be unused — no weak-password attempt should consume it
	var usedAt *time.Time
	require.NoError(t, testPool.QueryRow(context.Background(),
		"SELECT used_at FROM user_invitations WHERE token = 'tok'",
	).Scan(&usedAt))
	assert.Nil(t, usedAt)
}

// --- ChangePassword ---

func TestChangePassword_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	sessionToken := mustCreateSession(t, userID)

	w := httptest.NewRecorder()
	changePasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/user/password", sessionToken,
		`{"current_password":"`+strongPassword+`","new_password":"NewP@ss1word"}`,
	))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "password changed", resp["message"])
}

func TestChangePassword_AllowsLoginWithNew(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	sessionToken := mustCreateSession(t, userID)

	w := httptest.NewRecorder()
	changePasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/user/password", sessionToken,
		`{"current_password":"`+strongPassword+`","new_password":"NewP@ss1word"}`,
	))
	require.Equal(t, http.StatusOK, w.Code)

	// Old password must no longer work
	w2 := postLogin(loginRouter(), `{"identifier":"alice","password":"`+strongPassword+`"}`)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	// New password must work
	w3 := postLogin(loginRouter(), `{"identifier":"alice","password":"NewP@ss1word"}`)
	require.Equal(t, http.StatusOK, w3.Code)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	sessionToken := mustCreateSession(t, userID)

	w := httptest.NewRecorder()
	changePasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/user/password", sessionToken,
		`{"current_password":"Wr0ng!pass","new_password":"NewP@ss1word"}`,
	))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	sessionToken := mustCreateSession(t, userID)

	tests := []struct {
		name     string
		password string
	}{
		{"too short", "Ab1!"},
		{"no uppercase", "newp@ss1word"},
		{"no lowercase", "NEWP@SS1WORD"},
		{"no digit", "NewP@ssword"},
		{"no special", "NewPass1word"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			changePasswordRouter().ServeHTTP(w, authedRequest(
				http.MethodPut, "/api/v1/user/password", sessionToken,
				`{"current_password":"`+strongPassword+`","new_password":"`+tc.password+`"}`,
			))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestChangePassword_RevokesOtherSessions(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	mustAddPermission(t, userID, perms.UserRead)

	// Create two sessions
	session1 := mustCreateSession(t, userID)
	session2 := mustCreateSession(t, userID)

	// Change password using session1
	w := httptest.NewRecorder()
	changePasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/user/password", session1,
		`{"current_password":"`+strongPassword+`","new_password":"NewP@ss1word"}`,
	))
	require.Equal(t, http.StatusOK, w.Code)

	// Both sessions must now be invalid
	w2 := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w2, authedRequest(http.MethodGet, "/api/v1/user", session1, ""))
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	w3 := httptest.NewRecorder()
	getUserRouter().ServeHTTP(w3, authedRequest(http.MethodGet, "/api/v1/user", session2, ""))
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}

func TestChangePassword_RequiresSession(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", strongPassword)
	mustAddPermission(t, userID, perms.UserRead)
	accessToken := mustCreateToken(t, userID, []string{"user.read"})

	w := httptest.NewRecorder()
	changePasswordRouter().ServeHTTP(w, authedRequest(
		http.MethodPut, "/api/v1/user/password", accessToken,
		`{"current_password":"`+strongPassword+`","new_password":"NewP@ss1word"}`,
	))

	// API tokens must be rejected — session required
	assert.Equal(t, http.StatusForbidden, w.Code)
}
