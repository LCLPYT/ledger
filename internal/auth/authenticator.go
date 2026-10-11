package auth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aarondl/opt/omitnull"
	"github.com/gofrs/uuid/v5"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/gen/go/bob/queries"
	"github.com/stephenafamo/bob"
)

var ErrInvalidToken = errors.New("auth: invalid token")

// Authenticator resolves API key tokens to principals.
// A resolved key is cached for TTL
type Authenticator struct {
	db  bob.DB
	ttl time.Duration

	mu    sync.Mutex
	cache map[uuid.UUID]cacheEntry
}

type cacheEntry struct {
	secretHash []byte
	principal  *Principal
	until      time.Time
}

func NewAuthenticator(db bob.DB, ttl time.Duration) *Authenticator {
	return &Authenticator{
		db:    db,
		ttl:   ttl,
		cache: make(map[uuid.UUID]cacheEntry),
	}
}

func (a *Authenticator) Authenticate(ctx context.Context, token string) (*Principal, error) {
	id, secret, ok := ParseToken(token)

	if !ok {
		return nil, ErrInvalidToken
	}

	hash := HashSecret(secret)
	now := time.Now()

	if p, ok := a.cached(id, hash, now); ok {
		return p, nil
	}

	key, err := model.FindAPIKey(ctx, a.db, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidToken
	}

	if err != nil {
		return nil, fmt.Errorf("auth: failed to look up api key %s: %w", id, err)
	}

	if subtle.ConstantTimeCompare(hash, key.SecretHash) != 1 || key.RevokeTime.IsValue() {
		return nil, ErrInvalidToken
	}

	until := now.Add(a.ttl)

	if expire, ok := key.ExpireTime.Get(); ok {
		if !now.Before(expire) {
			return nil, ErrInvalidToken
		}

		if expire.Before(until) {
			until = expire
		}
	}

	owner, err := model.FindUser(ctx, a.db, key.UserID)

	if err != nil {
		return nil, fmt.Errorf("auth: failed to look up the owner of api key %s: %w", id, err)
	}

	if owner.DeleteTime.IsValue() {
		return nil, ErrInvalidToken
	}

	permissions, err := queries.ApiKeyPermissions(id).All(ctx, a.db)

	if err != nil {
		return nil, fmt.Errorf("auth: failed to load the permissions of api key %s: %w", id, err)
	}

	scopes, err := queries.ApiKeyScopes(id).All(ctx, a.db)

	if err != nil {
		return nil, fmt.Errorf("auth: failed to load the scopes of api key %s: %w", id, err)
	}

	if err := key.Update(ctx, a.db, &model.APIKeySetter{LastUsedTime: omitnull.From(now)}); err != nil {
		return nil, fmt.Errorf("auth: failed to record the use of api key %s: %w", id, err)
	}

	p := &Principal{
		KeyID:       id,
		Name:        key.Name,
		Permissions: permissions,
		Scopes:      scopes,
	}

	a.mu.Lock()

	a.cache[id] = cacheEntry{
		secretHash: hash,
		principal:  p,
		until:      until,
	}

	a.mu.Unlock()

	return p, nil
}

func (a *Authenticator) cached(id uuid.UUID, hash []byte, now time.Time) (*Principal, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.cache[id]

	if !ok {
		return nil, false
	}

	if !now.Before(entry.until) {
		delete(a.cache, id)
		return nil, false
	}

	// A wrong secret for a cached key falls through to the database, which rejects it.
	if !bytes.Equal(entry.secretHash, hash) {
		return nil, false
	}

	return entry.principal, true
}
