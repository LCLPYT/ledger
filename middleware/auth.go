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

		c.Set("userID", claims.UserID)

		if claims.Type == auth.TypeAccessToken {
			HandleAccessTokenAuth(c, db, claims, requiredPermissions, enforcer)
			return
		}

		if claims.Type == auth.TypeSession {
			HandleSessionTokenAuth(c, requiredPermissions, enforcer)
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
		return
	}
}

func HandleSessionTokenAuth(c *gin.Context, requiredPermissions []string, enforcer *casbin.Enforcer) {
	userID := c.GetString("userID")

	for _, permission := range requiredPermissions {
		parts := strings.SplitN(permission, ".", 2)
		if len(parts) != 2 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		obj, act := parts[0], parts[1]

		ok, _ := enforcer.Enforce(userID, obj, act, permission)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	c.Next()
}

func HandleAccessTokenAuth(
	c *gin.Context,
	db *sql.DB,
	claims *auth.Claims,
	requiredPermissions []string,
	enforcer *casbin.Enforcer,
) {
	var scopesJson []byte
	var revoked bool
	err := db.QueryRow(
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

	for _, permission := range requiredPermissions {
		parts := strings.SplitN(permission, ".", 2)
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
