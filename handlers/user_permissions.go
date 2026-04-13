package handlers

import (
	"ledger/auth"
	"ledger/perms"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

func GetUserPermissions(enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID    := c.GetString("userID")
		tokenType := c.GetString("tokenType")

		var permissions []string

		if tokenType == auth.TypeSession {
			// Check each known permission via Enforce — handles wildcards (*.*,
			// obj.*) and transitive roles correctly without parsing raw policies.
			for _, perm := range perms.All {
				parts := strings.SplitN(perm, ".", 2)
				if len(parts) != 2 {
					continue
				}
				obj, act := parts[0], parts[1]
				ok, _ := enforcer.Enforce(userID, obj, act, perm)
				if ok {
					permissions = append(permissions, perm)
				}
			}
		} else {
			// access token: scopes set by HandleAccessTokenAuth
			permissions = c.GetStringSlice("tokenScopes")
		}

		if permissions == nil {
			permissions = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"permissions": permissions})
	}
}
