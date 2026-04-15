package handlers_test

import (
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ledger/auth"
	appdb "ledger/db"
	"ledger/handlers"

	"github.com/casbin/casbin/v2"
	"golang.org/x/crypto/bcrypt"
)

var (
	testDB       *sql.DB
	testEnforcer *casbin.Enforcer
)

func TestMain(m *testing.M) {
	// Prevent tests from making real Mojang API calls.
	handlers.FetchUsername = func(string) (string, error) { return "", nil }
	handlers.FetchUUIDByName = func(string) (string, error) { return "", nil }

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		dsn = "postgres://db:db@localhost:5433/db?sslmode=disable"
	}

	testDB = appdb.InitDB(dsn)
	testEnforcer = appdb.InitCasbin(dsn)

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE access_tokens, sessions, users, user_invitations, roles, coin_transactions, coin_balances, minecraft_players RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("cleanDB: %v", err)
	}
	testEnforcer.ClearPolicy()
}

func mustCreateSession(t *testing.T, userID int64) string {
	t.Helper()
	token, err := auth.GenerateSessionToken(strconv.FormatInt(userID, 10), testDB)
	if err != nil {
		t.Fatalf("mustCreateSession: %v", err)
	}
	return token
}

func mustCreateRole(t *testing.T, name string) {
	t.Helper()
	_, err := testDB.Exec("INSERT INTO roles (name) VALUES ($1)", name)
	if err != nil {
		t.Fatalf("mustCreateRole: %v", err)
	}
}

func mustCreateUser(t *testing.T, username, email, password string) int64 {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("mustCreateUser hash: %v", err)
	}
	var id int64
	err = testDB.QueryRow(
		"INSERT INTO users (username, email, password_hash, verified_at) VALUES ($1, $2, $3, now()) RETURNING id",
		username, email, hash,
	).Scan(&id)
	if err != nil {
		t.Fatalf("mustCreateUser insert: %v", err)
	}
	return id
}

// mustCreateShellUser inserts a user without a password or verified_at,
// as created by the invitation flow before the user sets their password.
func mustCreateShellUser(t *testing.T, username, email string) int64 {
	t.Helper()
	var id int64
	err := testDB.QueryRow(
		"INSERT INTO users (username, email) VALUES ($1, $2) RETURNING id",
		username, email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("mustCreateShellUser: %v", err)
	}
	return id
}

func mustCreateInvitation(t *testing.T, userID int64, token string, expiresAt time.Time) {
	t.Helper()
	_, err := testDB.Exec(
		"INSERT INTO user_invitations (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, token, expiresAt,
	)
	if err != nil {
		t.Fatalf("mustCreateInvitation: %v", err)
	}
}

func mustAddPermission(t *testing.T, userID int64, permission string) {
	parts := strings.Split(permission, ".")

	mustAddPolicy(t, userID, parts[0], parts[1])
}

func mustAddPolicy(t *testing.T, userID int64, obj, act string) {
	t.Helper()
	if _, err := testEnforcer.AddPolicy(strconv.FormatInt(userID, 10), obj, act); err != nil {
		t.Fatalf("mustAddPolicy: %v", err)
	}
}

func mustAddRolePermission(t *testing.T, roleName, permission string) {
	t.Helper()
	parts := strings.Split(permission, ".")
	if _, err := testEnforcer.AddPolicy(roleName, parts[0], parts[1]); err != nil {
		t.Fatalf("mustAddRolePermission: %v", err)
	}
}

func mustAssignUserToRole(t *testing.T, userID int64, roleName string) {
	t.Helper()
	if _, err := testEnforcer.AddGroupingPolicy(strconv.FormatInt(userID, 10), roleName); err != nil {
		t.Fatalf("mustAssignUserToRole: %v", err)
	}
}

func mustCreateToken(t *testing.T, userID int64, scopes []string) string {
	t.Helper()
	token, err := auth.GenerateToken(strconv.FormatInt(userID, 10), scopes, time.Now().Add(time.Hour), testDB, "test")
	if err != nil {
		t.Fatalf("mustCreateToken: %v", err)
	}
	return token
}
