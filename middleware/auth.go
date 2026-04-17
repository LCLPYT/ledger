package middleware

import (
	"context"
	"encoding/json"
	"ledger/auth"
	dbsqlc "ledger/db/sqlc"
	"net/http"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func AuthRequired(enforcer *casbin.Enforcer, pool *pgxpool.Pool, requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, done := DecodeJwt(c)
		if done {
			return
		}

		if claims.Type == auth.TypeAccessToken {
			HandleAccessTokenAuth(c, pool, claims, requiredPermissions, enforcer)
			return
		}

		if claims.Type == auth.TypeSession {
			HandleSessionTokenAuth(c, pool, requiredPermissions, enforcer)
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
	}
}

func SessionRequired(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, done := DecodeJwt(c)
		if done {
			return
		}

		if claims.Type != auth.TypeSession {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not in a session"})
			return
		}

		if !sessionExists(pool, claims.RegisteredClaims.ID, claims.UserID) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or revoked"})
			return
		}

		c.Next()
	}
}

func NotAuthenticated(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "only available when not authenticated"})
		return
	}

	c.Next()
}

func sessionExists(pool *pgxpool.Pool, sessionID, userID string) bool {
	sid, err := strconv.ParseInt(sessionID, 10, 64)
	if err != nil {
		return false
	}
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false
	}

	q := dbsqlc.New(pool)
	exists, err := q.SessionExists(context.Background(), dbsqlc.SessionExistsParams{
		ID:     sid,
		UserID: uid,
	})
	return err == nil && exists
}

func DecodeJwt(c *gin.Context) (*auth.Claims, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, true
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &auth.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return auth.JwtKey, nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return nil, true
	}

	c.Set("userID", claims.UserID)
	c.Set("sessionID", claims.RegisteredClaims.ID)
	c.Set("tokenType", claims.Type)
	c.Set("tokenID", claims.TokenId)
	return claims, false
}

func HandleSessionTokenAuth(c *gin.Context, pool *pgxpool.Pool, requiredPermissions []string, enforcer *casbin.Enforcer) {
	userID := c.GetString("userID")
	sessionID := c.GetString("sessionID")

	if !sessionExists(pool, sessionID, userID) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired or revoked"})
		return
	}

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
	pool *pgxpool.Pool,
	claims *auth.Claims,
	requiredPermissions []string,
	enforcer *casbin.Enforcer,
) {
	tokenID, err := strconv.ParseInt(claims.TokenId, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	userID, err := strconv.ParseInt(claims.UserID, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	q := dbsqlc.New(pool)
	row, err := q.GetTokenAuth(c.Request.Context(), dbsqlc.GetTokenAuthParams{
		ID:     tokenID,
		UserID: userID,
	})
	if err != nil || row.Revoked.Bool {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	var scopes []string
	if err := json.Unmarshal(row.Scopes, &scopes); err != nil {
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

	c.Set("tokenScopes", scopes)
	c.Next()
}
