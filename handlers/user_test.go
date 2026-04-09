package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	mustAddPolicy(t, userID, "user", "read")

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
	mustAddPolicy(t, userID, "user", "read")

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
