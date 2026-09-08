package main

import (
	"context"
	"ledger/wiring"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
)

const envPrefix = "LEDGER__"

func main() {
	//if err := cmd.Execute(); err != nil {
	//	log.Fatal(err)
	//}

	if err := run(); err != nil {
		slog.Error("service failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	unregister := context.AfterFunc(ctx, stop)
	defer unregister()

	conf := config.Module(config.Options{EnvPrefix: envPrefix})

	a, err := app.New(ctx, wiring.Modules(conf))
	if err != nil {
		return err
	}

	return a.Run(ctx)
}
