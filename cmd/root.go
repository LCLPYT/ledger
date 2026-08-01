package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ledger",
	Short: "Ledger",
}

func Execute() error {
	return rootCmd.Execute()
}
