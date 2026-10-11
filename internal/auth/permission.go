package auth

import (
	"regexp"
	"strings"
)

const maxPermissionLength = 255

// permissionPattern mirrors the check constraint on role_permissions.permission.
// A permission reads <resource>.<action>[:<scope>]
// a trailing * grants everything below it, e.g. "games.*", "assets.read:*" or "*".
var permissionPattern = regexp.MustCompile(`^(\*|[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*(\.\*|:\*|:[a-z0-9_-]+)?)$`)

func ValidPermission(permission string) bool {
	return len(permission) <= maxPermissionLength && permissionPattern.MatchString(permission)
}

// Grants reports whether the granted permission covers the required one.
func Grants(granted, required string) bool {
	if granted == required {
		return true
	}

	prefix, wildcard := strings.CutSuffix(granted, "*")

	return wildcard && strings.HasPrefix(required, prefix)
}
