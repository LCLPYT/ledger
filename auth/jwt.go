package auth

import (
	"database/sql"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var JwtKey = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	UserID  string `json:"user_id"`
	TokenId string `json:"token_id"`
	jwt.RegisteredClaims
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
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(JwtKey)
}
