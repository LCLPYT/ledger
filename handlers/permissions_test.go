package handlers_test

import (
	"encoding/json"
	"ledger/handlers"
	"ledger/middleware"
	"ledger/permissions"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listPermissionsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/permissions",
		middleware.AuthRequired(testEnforcer, testDB, permissions.PermissionsList),
		handlers.ListPermissions())
	return r
}

func TestListPermissions_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "permissions", "list")
	token := mustCreateToken(t, userID, []string{permissions.PermissionsList})

	w := httptest.NewRecorder()
	listPermissionsRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/permissions", token, ""))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string][]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.ElementsMatch(t, permissions.All, resp["permissions"])
}

func TestListPermissions_Unauthenticated(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	listPermissionsRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/permissions", "", ""))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListPermissions_Forbidden(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "user", "read")
	token := mustCreateToken(t, userID, []string{permissions.UserRead})

	w := httptest.NewRecorder()
	listPermissionsRouter().ServeHTTP(w, authedRequest(http.MethodGet, "/api/v1/permissions", token, ""))

	assert.Equal(t, http.StatusForbidden, w.Code)
}
