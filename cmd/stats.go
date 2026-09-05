package cmd

import (
	"fmt"

	"ledger/stats"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Maintains the recorded game statistics",
}

func init() {
	statsCmd.AddCommand(statsRebuildCmd)

	rootCmd.AddCommand(statsCmd)
}

var statsRebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Recompute records and aggregates from the recorded matches",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := ConnectCmdDatabase()

		if err != nil {
			return err
		}

		defer db.Close()

		if err := stats.NewService(db).Rebuild(cmd.Context()); err != nil {
			return err
		}

		fmt.Println("Rebuilt personal records and player aggregates")

		return nil
	},
}
