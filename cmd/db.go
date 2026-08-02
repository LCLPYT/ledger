package cmd

import (
	"fmt"
	"strings"

	"ledger/config"
	"ledger/db"
	"ledger/db/migrations"

	"github.com/spf13/cobra"
	"github.com/uptrace/bun/migrate"
)

/*
Ported from bun example:
https://github.com/uptrace/bun/blob/0e5e8682686253e24add0e8512d351d5e32dd9bf/example/migrate/main.go
*/

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Handles database migrations",
}

func init() {
	dbCmd.AddCommand(
		dbInitCmd,
		dbMigrateCmd,
		dbRollbackCmd,
		dbLockCmd,
		dbUnlockCmd,
		dbCreateGoCmd,
		dbCreateSQLCmd,
		dbCreateTxSQLCmd,
		dbStatusCmd,
		dbMarkAppliedCmd,
	)

	rootCmd.AddCommand(dbCmd)
}

func newMigrator() (*migrate.Migrator, error) {
	v, err := config.CreateCommonConfig()
	if err != nil {
		return nil, err
	}

	return migrate.NewMigrator(db.ConnectDatabase(v), migrations.Migrations), nil
}

var dbInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create migration tables",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		return migrator.Init(cmd.Context())
	},
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate database",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		if err := migrator.Lock(cmd.Context()); err != nil {
			return err
		}

		defer migrator.Unlock(cmd.Context())

		group, err := migrator.Migrate(cmd.Context())

		if err != nil {
			return err
		}

		if group.IsZero() {
			fmt.Printf("There are no new migrations to run (database is up to date)\n")
			return nil
		}

		fmt.Printf("Migrated to %s\n", group)
		return nil
	},
}

var dbRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback the last migration group",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		if err := migrator.Lock(cmd.Context()); err != nil {
			return err
		}

		defer migrator.Unlock(cmd.Context())

		group, err := migrator.Rollback(cmd.Context())

		if err != nil {
			return err
		}

		if group.IsZero() {
			fmt.Printf("There are no groups to roll back\n")
			return nil
		}

		fmt.Printf("Rolled back %s\n", group)
		return nil
	},
}

var dbLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		return migrator.Lock(cmd.Context())
	},
}

var dbUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		return migrator.Unlock(cmd.Context())
	},
}

var dbCreateGoCmd = &cobra.Command{
	Use:   "create_go",
	Short: "Create Go migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		mf, err := migrator.CreateGoMigration(cmd.Context(), name)
		if err != nil {
			return err
		}

		fmt.Printf("created migration %s (%s)\n", mf.Name, mf.Path)
		return nil
	},
}

var dbCreateSQLCmd = &cobra.Command{
	Use:   "create_sql",
	Short: "Create up and down SQL migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		files, err := migrator.CreateSQLMigrations(cmd.Context(), name)
		if err != nil {
			return err
		}

		for _, mf := range files {
			fmt.Printf("Created migration %s (%s)\n", mf.Name, mf.Path)
		}

		return nil
	},
}

var dbCreateTxSQLCmd = &cobra.Command{
	Use:   "create_tx_sql",
	Short: "Create up and down transactional SQL migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		files, err := migrator.CreateTxSQLMigrations(cmd.Context(), name)
		if err != nil {
			return err
		}

		for _, mf := range files {
			fmt.Printf("created transaction migration %s (%s)\n", mf.Name, mf.Path)
		}

		return nil
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print migrations status",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		ms, err := migrator.MigrationsWithStatus(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("Migrations: %s\n", ms)
		fmt.Printf("Unapplied migrations: %s\n", ms.Unapplied())
		fmt.Printf("Last migration group: %s\n", ms.LastGroup())
		return nil
	},
}

var dbMarkAppliedCmd = &cobra.Command{
	Use:   "mark_applied",
	Short: "Mark migrations as applied without actually running them",
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := newMigrator()
		if err != nil {
			return err
		}

		group, err := migrator.Migrate(cmd.Context(), migrate.WithNopMigration())
		if err != nil {
			return err
		}

		if group.IsZero() {
			fmt.Printf("There are no new migrations to mark as applied\n")
			return nil
		}

		fmt.Printf("Marked as applied %s\n", group)
		return nil
	},
}
