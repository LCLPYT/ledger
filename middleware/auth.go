package middleware

import (
	"database/sql"
	"encoding/json"
	"ledger/auth"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthRequired(enforcer *casbin.Enforcer, db *sql.DB, requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return auth.JwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		var scopesJson []byte
		var revoked bool
		err = db.QueryRow(
			"SELECT scopes, revoked FROM access_tokens WHERE id = $1 AND user_id = $2 AND expires_at > now()",
			claims.TokenId, claims.UserID,
		).Scan(&scopesJson, &revoked)

		if err != nil || revoked {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		var scopes []string
		if err := json.Unmarshal(scopesJson, &scopes); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set("userID", claims.UserID)

		for _, required := range requiredPermissions {
			parts := strings.SplitN(required, ".", 2)
			if len(parts) != 2 {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			obj, act := parts[0], parts[1]

			granted := false
			for _, scope := range scopes {
				ok, _ := enforcer.Enforce(claims.UserID, obj, act, scope)
				if ok {
					granted = true
					break
				}
			}

			if !granted {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		c.Next()
	}
}
