package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/quadrubo/golib/app"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

const shutdownTimeout = 5 * time.Second

var rootCmd = &cobra.Command{
	Use:          "ledger",
	Short:        "Ledger",
	SilenceUsage: true,
}

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

// withApp boots the modules, hands the injector to fn and tears the app down again.
// fn may be nil for a command whose work is a module's Provide.
func withApp(ctx context.Context, modules []app.Module, fn func(context.Context, do.Injector) error) error {
	a, err := app.New(ctx, modules)

	if err != nil {
		return err
	}

	var runErr error

	if fn != nil {
		runErr = fn(ctx, a.Injector())
	}

	// Shutdown gets a context of its own so a canceled command still releases what the modules hold.
	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return errors.Join(runErr, a.Shutdown(stopCtx))
}
