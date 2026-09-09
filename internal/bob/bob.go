package bob

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/quadrubo/golib/app"
	"github.com/samber/do/v2"
	"github.com/stephenafamo/bob"
)

func Module() app.Module {
	return &module{}
}

type module struct {
	db bob.DB
}

var _ app.Provider = (*module)(nil)

func (m *module) Name() string { return "bob" }

func (m *module) Provide(_ context.Context, i do.Injector) error {
	db, err := do.Invoke[*sql.DB](i)
	if err != nil {
		return fmt.Errorf("bob: failed to obtain database: %w", err)
	}

	m.db = bob.NewDB(db)

	do.ProvideValue(i, m.db)

	return nil
}

func (m *module) Stop() error {
	err := m.db.Close()
	if err != nil {
		return fmt.Errorf("bob: failed to close database: %w", err)
	}

	return nil
}
