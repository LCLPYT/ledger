package handlers_test

import (
	"encoding/json"
	"fmt"
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

func createRoleRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/roles",
		middleware.AuthRequired(testEnforcer, testDB, permissions.RolesCreate),
		handlers.CreateRole(testDB))
	return r
}

func roleUsersRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/roles/:role/users",
		middleware.AuthRequired(testEnforcer, testDB, permissions.RolesManageUsers),
		handlers.AddUserToRole(testDB, testEnforcer))
	r.DELETE("/api/v1/roles/:role/users",
		middleware.AuthRequired(testEnforcer, testDB, permissions.RolesManageUsers),
		handlers.RemoveUserFromRole(testDB, testEnforcer))
	return r
}

func TestCreateRole_Success(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "roles", "create")
	token := mustCreateToken(t, userID, []string{permissions.RolesCreate})

	w := httptest.NewRecorder()
	createRoleRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles", token, `{"name":"admin"}`))

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "admin", resp["name"])
	assert.NotEmpty(t, resp["id"])
}

func TestCreateRole_Duplicate(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "roles", "create")
	token := mustCreateToken(t, userID, []string{permissions.RolesCreate})
	mustCreateRole(t, "admin")

	w := httptest.NewRecorder()
	createRoleRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles", token, `{"name":"admin"}`))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateRole_MissingName(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "roles", "create")
	token := mustCreateToken(t, userID, []string{permissions.RolesCreate})

	w := httptest.NewRecorder()
	createRoleRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles", token, `{}`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRole_Unauthenticated(t *testing.T) {
	cleanDB(t)

	w := httptest.NewRecorder()
	createRoleRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles", "", `{"name":"admin"}`))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateRole_Forbidden(t *testing.T) {
	cleanDB(t)
	userID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, userID, "user", "read")
	token := mustCreateToken(t, userID, []string{permissions.UserRead})

	w := httptest.NewRecorder()
	createRoleRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles", token, `{"name":"admin"}`))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddUserToRole_Success(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})
	targetID := mustCreateUser(t, "bob", "bob@example.com", "x")
	mustCreateRole(t, "viewer")

	body := fmt.Sprintf(`{"user_id":"%d"}`, targetID)
	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles/viewer/users", token, body))

	require.Equal(t, http.StatusOK, w.Code)
	roles, err := testEnforcer.GetRolesForUser(fmt.Sprintf("%d", targetID))
	require.NoError(t, err)
	assert.Contains(t, roles, "viewer")
}

func TestAddUserToRole_RoleNotFound(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})
	targetID := mustCreateUser(t, "bob", "bob@example.com", "x")

	body := fmt.Sprintf(`{"user_id":"%d"}`, targetID)
	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles/nonexistent/users", token, body))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddUserToRole_AlreadyAssigned(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})
	targetID := mustCreateUser(t, "bob", "bob@example.com", "x")
	mustCreateRole(t, "viewer")
	_, err := testEnforcer.AddGroupingPolicy(fmt.Sprintf("%d", targetID), "viewer")
	require.NoError(t, err)

	body := fmt.Sprintf(`{"user_id":"%d"}`, targetID)
	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles/viewer/users", token, body))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAddUserToRole_Unauthenticated(t *testing.T) {
	cleanDB(t)
	mustCreateRole(t, "viewer")

	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodPost, "/api/v1/roles/viewer/users", "", `{"user_id":"1"}`))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRemoveUserFromRole_Success(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})
	targetID := mustCreateUser(t, "bob", "bob@example.com", "x")
	mustCreateRole(t, "viewer")
	_, err := testEnforcer.AddGroupingPolicy(fmt.Sprintf("%d", targetID), "viewer")
	require.NoError(t, err)

	body := fmt.Sprintf(`{"user_id":"%d"}`, targetID)
	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodDelete, "/api/v1/roles/viewer/users", token, body))

	require.Equal(t, http.StatusOK, w.Code)
	roles, err := testEnforcer.GetRolesForUser(fmt.Sprintf("%d", targetID))
	require.NoError(t, err)
	assert.NotContains(t, roles, "viewer")
}

func TestRemoveUserFromRole_RoleNotFound(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})

	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodDelete, "/api/v1/roles/nonexistent/users", token, `{"user_id":"1"}`))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveUserFromRole_NotAssigned(t *testing.T) {
	cleanDB(t)
	callerID := mustCreateUser(t, "alice", "alice@example.com", "x")
	mustAddPolicy(t, callerID, "roles", "manage_users")
	token := mustCreateToken(t, callerID, []string{permissions.RolesManageUsers})
	targetID := mustCreateUser(t, "bob", "bob@example.com", "x")
	mustCreateRole(t, "viewer")

	body := fmt.Sprintf(`{"user_id":"%d"}`, targetID)
	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodDelete, "/api/v1/roles/viewer/users", token, body))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveUserFromRole_Unauthenticated(t *testing.T) {
	cleanDB(t)
	mustCreateRole(t, "viewer")

	w := httptest.NewRecorder()
	roleUsersRouter().ServeHTTP(w, authedRequest(http.MethodDelete, "/api/v1/roles/viewer/users", "", `{"user_id":"1"}`))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
