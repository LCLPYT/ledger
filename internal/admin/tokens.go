package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/auth"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/gofrs/uuid/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
)

type TokenStatus string

const (
	TokenActive       TokenStatus = "active"
	TokenExpired      TokenStatus = "expired"
	TokenRevoked      TokenStatus = "revoked"
	TokenOwnerDeleted TokenStatus = "owner deleted"
)

type Token struct {
	*model.APIKey
	Owner  *model.User
	Scopes []string
}

func (t Token) Status(now time.Time) TokenStatus {
	switch {
	case t.RevokeTime.IsValue():
		return TokenRevoked
	case t.Owner.DeleteTime.IsValue():
		return TokenOwnerDeleted
	case t.ExpireTime.IsValue() && !now.Before(t.ExpireTime.MustGet()):
		return TokenExpired
	default:
		return TokenActive
	}
}

func (s *Service) Tokens(ctx context.Context) ([]Token, error) {
	keys, err := model.APIKeys.Query(
		sm.OrderBy(model.APIKeys.Columns.CreateTime).Desc(),
		model.Preload.APIKey.User(),
	).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	scopes, err := model.APIKeyScopes.Query(sm.OrderBy(model.APIKeyScopes.Columns.Permission)).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	scopesByKey := make(map[uuid.UUID][]string)

	for _, scope := range scopes {
		scopesByKey[scope.APIKeyID] = append(scopesByKey[scope.APIKeyID], scope.Permission)
	}

	result := make([]Token, len(keys))

	for i, key := range keys {
		result[i] = Token{APIKey: key, Owner: key.R.User, Scopes: scopesByKey[key.ID]}
	}

	return result, nil
}

// UserPermissions returns the permissions every user holds through their roles.
func (s *Service) UserPermissions(ctx context.Context) (map[int64][]string, error) {
	memberships, err := model.UserRoles.Query().All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	permissions, err := model.RolePermissions.Query().All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	byRole := make(map[string][]string)

	for _, p := range permissions {
		byRole[p.RoleName] = append(byRole[p.RoleName], p.Permission)
	}

	sets := make(map[int64]map[string]struct{})

	for _, m := range memberships {
		if sets[m.UserID] == nil {
			sets[m.UserID] = make(map[string]struct{})
		}

		for _, p := range byRole[m.RoleName] {
			sets[m.UserID][p] = struct{}{}
		}
	}

	result := make(map[int64][]string, len(sets))

	for id, set := range sets {
		result[id] = slices.Sorted(maps.Keys(set))
	}

	return result, nil
}

// Scopes returns the permissions a token of a user holding granted can be restricted to.
func (s *Service) Scopes(granted []string) []string {
	options := slices.DeleteFunc(s.PermissionOptions(), func(p string) bool {
		return !auth.GrantsAny(granted, p)
	})

	return slices.Compact(slices.Sorted(slices.Values(append(options, granted...))))
}

// CreateToken returns the token, which cannot be recovered later on. An empty scope list leaves the token unrestricted.
func (s *Service) CreateToken(
	ctx context.Context,
	ownerID int64,
	name string,
	expire *time.Time,
	scopes []string,
) (string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return "", errors.New("name must not be empty")
	}

	if err := ValidatePermissions(scopes); err != nil {
		return "", err
	}

	owner, err := findUser(ctx, s.db, ownerID)

	if err != nil {
		return "", err
	}

	if owner.DeleteTime.IsValue() {
		return "", fmt.Errorf("user %s is deleted", owner.Name)
	}

	token, id, hash, err := auth.NewToken()

	if err != nil {
		return "", err
	}

	err = s.inTx(ctx, func(tx bob.Executor) error {
		_, err := model.APIKeys.Insert(&model.APIKeySetter{
			ID:         omit.From(id),
			Name:       omit.From(name),
			SecretHash: omit.From(hash),
			ExpireTime: omitnull.FromPtr(expire),
			UserID:     omit.From(ownerID),
		}).One(ctx, tx)

		if err != nil || len(scopes) == 0 {
			return err
		}

		scopes = slices.Compact(slices.Sorted(slices.Values(scopes)))
		setters := make([]bob.Mod[*dialect.InsertQuery], len(scopes))

		for i, scope := range scopes {
			setters[i] = &model.APIKeyScopeSetter{APIKeyID: omit.From(id), Permission: omit.From(scope)}
		}

		_, err = model.APIKeyScopes.Insert(setters...).Exec(ctx, tx)

		return err
	})

	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) RevokeToken(ctx context.Context, id uuid.UUID) error {
	key, err := findToken(ctx, s.db, id)

	if err != nil {
		return err
	}

	return key.Update(ctx, s.db, &model.APIKeySetter{RevokeTime: omitnull.From(time.Now())})
}

func (s *Service) DeleteToken(ctx context.Context, id uuid.UUID) error {
	key, err := findToken(ctx, s.db, id)

	if err != nil {
		return err
	}

	return key.Delete(ctx, s.db)
}

func findToken(ctx context.Context, exec bob.Executor, id uuid.UUID) (*model.APIKey, error) {
	key, err := model.FindAPIKey(ctx, exec, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("token %s: %w", id, ErrNotFound)
	}

	return key, err
}
