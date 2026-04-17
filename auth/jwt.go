package auth

import (
	"context"
	"encoding/json"
	dbsqlc "ledger/db/sqlc"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
func GenerateSessionToken(userID int64, pool *pgxpool.Pool) (string, error) {
	expiry := time.Now().Add(SessionLifetime)

	q := dbsqlc.New(pool)
	sessionID, err := q.InsertSession(context.Background(), dbsqlc.InsertSessionParams{
		UserID:    userID,
		ExpiresAt: pgtype.Timestamp{Time: expiry, Valid: true},
	})
	if err != nil {
		return "", err
	}

	claims := &Claims{
		UserID:  strconv.FormatInt(userID, 10),
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

func GenerateToken(userID string, scopes []string, expiry time.Time, pool *pgxpool.Pool, name string) (string, error) {
	scopesJson, err := json.Marshal(scopes)
	if err != nil {
		return "", err
	}

	userIDInt, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", err
	}

	q := dbsqlc.New(pool)
	tokenId, err := q.InsertToken(context.Background(), dbsqlc.InsertTokenParams{
		UserID:    userIDInt,
		Name:      name,
		ExpiresAt: pgtype.Timestamp{Time: expiry, Valid: true},
		Scopes:    scopesJson,
	})
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
