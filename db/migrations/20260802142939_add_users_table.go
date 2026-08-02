package migrations

import (
	"context"
	"fmt"

	"ledger/model"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up] add users table\n")
		_, err := db.NewCreateTable().
			Model((*model.User)(nil)).
			IfNotExists().
			WithForeignKeys().
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down] drop users table\n")
		_, err := db.NewDropTable().
			Model((*model.User)(nil)).
			IfExists().
			Exec(ctx)
		return err
	})
}
