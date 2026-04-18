package handlers_test

import (
	"context"
	"ledger/mc"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ledger/auth"
	appdb "ledger/db"

	"github.com/casbin/casbin/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	testPool     *pgxpool.Pool
	testEnforcer *casbin.Enforcer
)

func TestMain(m *testing.M) {
	// Prevent tests from making real Mojang API calls.
	mc.FetchUsername = func(string) (string, error) { return "", nil }
	mc.FetchUUIDByName = func(string) (string, error) { return "", nil }

	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		dsn = "postgres://db:db@localhost:5433/db?sslmode=disable"
	}

	pool, cancel := appdb.InitDB(dsn)
	defer cancel()

	testPool = pool

	testEnforcer = appdb.InitCasbin(dsn)

	code := m.Run()
	os.Exit(code)
}

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE
			access_tokens,
			sessions,
			users,
			user_invitations,
			roles,
			coin_transactions,
			coin_balances,
			minecraft_players
			RESTART IDENTITY CASCADE`,
	)
	if err != nil {
		t.Fatalf("cleanDB: %v", err)
	}
	testEnforcer.ClearPolicy()
	if err := testEnforcer.SavePolicy(); err != nil {
		t.Fatalf("cleanDB SavePolicy: %v", err)
	}
}

func mustCreateSession(t *testing.T, userID int64) string {
	t.Helper()
	token, err := auth.GenerateSessionToken(userID, testPool)
	if err != nil {
		t.Fatalf("mustCreateSession: %v", err)
	}
	return token
}

func mustCreateRole(t *testing.T, name string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "INSERT INTO roles (name) VALUES ($1)", name)
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
	err = testPool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, password_hash, verified_at)
		VALUES ($1, $2, $3, now()) RETURNING id`,
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
	err := testPool.QueryRow(context.Background(),
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
	_, err := testPool.Exec(context.Background(),
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
	_, err := testEnforcer.AddPolicy(strconv.FormatInt(userID, 10), obj, act)
	if err != nil && err.Error() != "policy already exists" {
		t.Fatalf("mustAddPolicy: %v", err)
	}
}

func mustAddRolePermission(t *testing.T, roleName, permission string) {
	t.Helper()
	parts := strings.Split(permission, ".")
	_, err := testEnforcer.AddPolicy(roleName, parts[0], parts[1])
	if err != nil && err.Error() != "policy already exists" {
		t.Fatalf("mustAddRolePermission: %v", err)
	}
}

func mustAssignUserToRole(t *testing.T, userID int64, roleName string) {
	t.Helper()
	_, err := testEnforcer.AddGroupingPolicy(strconv.FormatInt(userID, 10), roleName)
	if err != nil && err.Error() != "policy already exists" {
		t.Fatalf("mustAssignUserToRole: %v", err)
	}
}

func mustCreateToken(t *testing.T, userID int64, scopes []string) string {
	t.Helper()
	token, err := auth.GenerateToken(strconv.FormatInt(userID, 10), scopes, time.Now().Add(time.Hour), testPool, "test")
	if err != nil {
		t.Fatalf("mustCreateToken: %v", err)
	}
	return token
}
