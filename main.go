package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lclpyt/ledger/internal/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	unregister := context.AfterFunc(ctx, stop)
	defer unregister()

	if err := cmd.Execute(ctx); err != nil {
		slog.Error("command failed", slog.Any("error", err))
		os.Exit(1)
	}
}
