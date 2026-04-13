package auth

import (
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const SessionLifetime = 1 * time.Hour

var JwtKey = []byte(os.Getenv("JWT_SECRET"))

const (
	TypeSession     = "session"
	TypeAccessToken = "token"
)

type Claims struct {
	UserID  string `json:"user_id"`
	TokenId string `json:"token_id"`
	Type    string `json:"type"`
	jwt.RegisteredClaims
}

// GenerateSessionToken generates a session token intended for graphical frontends.
// Session tokens implicitly grant all permissions that the user has.
// Session tokens are stored in the database and can be revoked.
func GenerateSessionToken(userID string, db *sql.DB) (string, error) {
	expiry := time.Now().Add(SessionLifetime)

	var sessionID int64
	err := db.QueryRow(
		"INSERT INTO sessions (user_id, expires_at) VALUES ($1, $2) RETURNING id",
		userID, expiry,
	).Scan(&sessionID)
	if err != nil {
		return "", err
	}

	claims := &Claims{
		UserID:  userID,
		TokenId: "",
		Type:    TypeSession,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        strconv.FormatInt(sessionID, 10),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(JwtKey)
}

// GenerateSessionTokenFromID re-signs a session JWT for an existing session row (used during refresh).
func GenerateSessionTokenFromID(userID string, sessionID string, expiry time.Time) (string, error) {
	claims := &Claims{
		UserID:  userID,
		TokenId: "",
		Type:    TypeSession,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JwtKey)
}

func GenerateToken(userID string, scopes []string, expiry time.Time, db *sql.DB, name string) (string, error) {
	scopesJson, err := json.Marshal(scopes)
	if err != nil {
		return "", err
	}

	var tokenId int64

	err = db.QueryRow(
		"INSERT INTO access_tokens (user_id, name, expires_at, scopes) VALUES ($1, $2, $3, $4) RETURNING id",
		userID, name, expiry, scopesJson,
	).Scan(&tokenId)

	if err != nil {
		return "", err
	}

	claims := &Claims{
		UserID:  userID,
		TokenId: strconv.FormatInt(tokenId, 10),
		Type:    TypeAccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(JwtKey)
}
