package handlers_test

import (
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	"ledger/auth"
	appdb "ledger/db"

	"github.com/casbin/casbin/v2"
	"golang.org/x/crypto/bcrypt"
)

var (
	testDB       *sql.DB
	testEnforcer *casbin.Enforcer
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	testDB = appdb.InitDB(dsn)
	testEnforcer = appdb.InitCasbin(dsn)

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE access_tokens, users, roles RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("cleanDB: %v", err)
	}
	testEnforcer.ClearPolicy()
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
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		username, email, hash,
	).Scan(&id)
	if err != nil {
		t.Fatalf("mustCreateUser insert: %v", err)
	}
	return id
}

func mustAddPolicy(t *testing.T, userID int64, obj, act string) {
	t.Helper()
	if _, err := testEnforcer.AddPolicy(strconv.FormatInt(userID, 10), obj, act); err != nil {
		t.Fatalf("mustAddPolicy: %v", err)
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
