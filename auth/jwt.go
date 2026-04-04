package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var JwtKey = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(userID string, permissions []string, expiry time.Time) (string, error) {
	claims := &Claims{
		UserID:      userID,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JwtKey)
}
