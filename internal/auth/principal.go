package auth

import (
	"context"

	"github.com/gofrs/uuid/v5"
)

// Principal is the authenticated caller of a request.
// Scopes narrow the permissions down further, no scopes = unrestricted.
type Principal struct {
	KeyID       uuid.UUID
	Name        string
	Permissions []string
	Scopes      []string
}

func (p *Principal) Can(permission string) bool {
	return grantsAny(p.Permissions, permission) && (len(p.Scopes) == 0 || grantsAny(p.Scopes, permission))
}

func grantsAny(granted []string, required string) bool {
	for _, g := range granted {
		if Grants(g, required) {
			return true
		}
	}

	return false
}

type principalKey struct{}

func NewContext(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)

	return p, ok
}
