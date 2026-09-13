package cmd

import (
	"fmt"

	"github.com/lclpyt/ledger/internal/wiring"
	"github.com/quadrubo/golib/app"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the ledger gRPC server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		a, err := app.New(ctx, wiring.Serve(wiring.Config()))
		if err != nil {
			return fmt.Errorf("cmd: error initializing application: %w", err)
		}

		return a.Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
