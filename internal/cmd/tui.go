package cmd

import (
	"context"

	"github.com/lclpyt/ledger/internal/admin"
	"github.com/lclpyt/ledger/internal/auth/permissions"
	"github.com/lclpyt/ledger/internal/tui"
	"github.com/lclpyt/ledger/internal/wiring"

	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
	"github.com/stephenafamo/bob"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Opens the interactive management app for users, roles and access tokens",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(cmd.Context(), wiring.Database(wiring.Config()), func(ctx context.Context, i do.Injector) error {
			db, err := do.Invoke[bob.DB](i)

			if err != nil {
				return err
			}

			knownPermissions, err := permissions.ListAvailable(i)

			if err != nil {
				return err
			}

			return tui.Run(ctx, admin.New(db, knownPermissions))
		})
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
