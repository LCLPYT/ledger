package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	api2 "github.com/lclpyt/ledger/internal/api"
	"github.com/lclpyt/ledger/internal/wiring"

	"github.com/gin-gonic/gin"
	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/logging"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the ledger HTTP server",
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

func serve(ctx context.Context, i do.Injector, addr string) error {
	log, err := logging.Component(i, "http")
	if err != nil {
		return err
	}

	router := gin.New()
	router.Use(gin.Logger(), api2.Recovery())

	if err := router.SetTrustedProxies(nil); err != nil {
		return fmt.Errorf("SetTrustedProxies: %w", err)
	}

	api2.SetupRoutes(router)

	s := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	listenErr := make(chan error, 1)

	go func() {
		log.Info("listening", slog.String("addr", addr))

		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
			return
		}

		listenErr <- nil
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Info("stopped")

	return nil
}
