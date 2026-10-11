package auth

import (
	"maps"
	"regexp"
	"slices"
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

func GrantsAny(granted []string, required string) bool {
	for _, g := range granted {
		if Grants(g, required) {
			return true
		}
	}

	return false
}

// WithWildcards adds every wildcard covering one of the permissions, e.g. "games.*" and "*" for "games.read".
func WithWildcards(permissions []string) []string {
	set := map[string]struct{}{"*": {}}

	for _, p := range permissions {
		set[p] = struct{}{}

		base, _, scoped := strings.Cut(p, ":")

		if scoped {
			set[base+":*"] = struct{}{}
		}

		for i := strings.LastIndexByte(base, '.'); i > 0; i = strings.LastIndexByte(base[:i], '.') {
			set[base[:i]+".*"] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(set))
}
