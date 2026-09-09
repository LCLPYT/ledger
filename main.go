package main

import (
	"context"
	"ledger/cmd"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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
