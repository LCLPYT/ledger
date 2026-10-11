// Package admin manages users, roles and access tokens. It backs the tui and headless commands alike.
package admin

import (
	"context"
	"errors"
	"slices"

	"github.com/lclpyt/ledger/internal/auth"

	"github.com/stephenafamo/bob"
)

var ErrNotFound = errors.New("not found")

type Service struct {
	db               bob.DB
	knownPermissions []string
}

func New(db bob.DB, knownPermissions []string) *Service {
	return &Service{
		db:               db,
		knownPermissions: knownPermissions,
	}
}

// PermissionOptions returns every permission option, including wildcards
func (s *Service) PermissionOptions() []string {
	return auth.WithWildcards(s.knownPermissions)
}

// UnknownPermissions filters the given permission slice and returns items that are not known by the app
func (s *Service) UnknownPermissions(permissions []string) []string {
	known := s.PermissionOptions()

	return slices.DeleteFunc(slices.Clone(permissions), func(p string) bool {
		return slices.Contains(known, p) || slices.ContainsFunc(s.knownPermissions, func(c string) bool {
			return auth.Grants(p, c)
		})
	})
}

func (s *Service) inTx(ctx context.Context, fn func(bob.Executor) error) error {
	tx, err := s.db.BeginTx(ctx, nil)

	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
