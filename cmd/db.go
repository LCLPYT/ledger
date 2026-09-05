package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ledger/config"
	"ledger/db"
	"ledger/db/migrations"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob"
)

const migrationsDir = "db/migrations"

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Handles database migrations",
}

func init() {
	dbCmd.AddCommand(
		dbMigrateCmd,
		dbRollbackCmd,
		dbStatusCmd,
		dbCreateCmd,
		dbDSNCmd,
	)

	rootCmd.AddCommand(dbCmd)
}

func ConnectCmdDatabase() (bob.DB, error) {
	v, err := config.CreateCommonConfig()

	if err != nil {
		return bob.DB{}, err
	}

	return db.ConnectDatabase(v)
}

func newProvider() (*goose.Provider, error) {
	v, err := config.CreateCommonConfig()

	if err != nil {
		return nil, err
	}

	sqldb, err := db.OpenSQL(v)

	if err != nil {
		return nil, err
	}

	locker, err := lock.NewPostgresSessionLocker()

	if err != nil {
		sqldb.Close()
		return nil, err
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqldb,
		migrations.FS,
		goose.WithSessionLocker(locker),
	)

	if err != nil {
		sqldb.Close()
		return nil, err
	}

	return provider, nil
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate database",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := newProvider()
		if err != nil {
			return err
		}
		defer provider.Close()

		results, err := provider.Up(cmd.Context())

		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Printf("There are no new migrations to run (database is up to date)\n")
			return nil
		}

		for _, result := range results {
			fmt.Printf("Migrated %s\n", filepath.Base(result.Source.Path))
		}

		return nil
	},
}

var dbRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back the last migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := newProvider()
		if err != nil {
			return err
		}
		defer provider.Close()

		result, err := provider.Down(cmd.Context())

		if err != nil {
			if errors.Is(err, goose.ErrNoNextVersion) {
				fmt.Printf("There are no migrations to roll back\n")
				return nil
			}

			return err
		}

		fmt.Printf("Rolled back %s\n", filepath.Base(result.Source.Path))
		return nil
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print migrations status",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := newProvider()
		if err != nil {
			return err
		}
		defer provider.Close()

		statuses, err := provider.Status(cmd.Context())

		if err != nil {
			return err
		}

		for _, status := range statuses {
			applied := "-"

			if !status.AppliedAt.IsZero() {
				applied = status.AppliedAt.Format(time.RFC3339)
			}

			fmt.Printf("%-16d %-40s %-8s %s\n",
				status.Source.Version,
				filepath.Base(status.Source.Path),
				status.State,
				applied,
			)
		}

		current, target, err := provider.GetVersions(cmd.Context())

		if err != nil {
			return err
		}

		fmt.Printf("Current version: %d (latest: %d)\n", current, target)
		return nil
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an up and down SQL migration",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")

		return goose.Create(nil, migrationsDir, name, "sql")
	},
}

var dbDSNCmd = &cobra.Command{
	Use:   "dsn",
	Short: "Print the database connection string",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := config.CreateCommonConfig()

		if err != nil {
			return err
		}

		dsn, err := db.DSN(v)

		if err != nil {
			return err
		}

		fmt.Println(dsn)
		return nil
	},
}
