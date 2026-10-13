package cmd

import (
	"github.com/lclpyt/ledger/internal/healthcheck"
	"github.com/lclpyt/ledger/internal/wiring"
	"github.com/quadrubo/golib/app"
	"github.com/spf13/cobra"
)

var healthcheckCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Probes the gRPC server and, when enabled, the asset repository of a running instance",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(cmd.Context(), []app.Module{wiring.Config()}, healthcheck.Probe)
	},
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}
