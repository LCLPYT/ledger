package auth

import (
	"context"

	"github.com/gofrs/uuid/v5"
)

// Principal is the authenticated caller of a request.
type Principal struct {
	KeyID       uuid.UUID
	Name        string
	Permissions []string
}

func (p *Principal) Can(permission string) bool {
	for _, granted := range p.Permissions {
		if Grants(granted, permission) {
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
