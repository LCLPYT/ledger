package cmd

import (
	"context"
	"errors"
	"fmt"
	"ledger/api"
	"ledger/config"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the ledger HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := setupServeConfig(cmd)

		if err != nil {
			return err
		}

		return serve(v)
	},
}

func init() {
	serveCmd.Flags().Int("port", 8080, "port to listen on")
	serveCmd.Flags().String("host", "localhost", "host to listen on")

	rootCmd.AddCommand(serveCmd)
}

func setupServeConfig(cmd *cobra.Command) (*viper.Viper, error) {
	v, err := config.CreateCommonConfig()
	if err != nil {
		return nil, err
	}

	v.SetDefault("host", "localhost")
	v.SetDefault("port", 8080)
	v.SetDefault("trusted_proxies", []string{})

	if err := v.BindPFlag("port", cmd.Flags().Lookup("port")); err != nil {
		return nil, err
	}

	if err := v.BindPFlag("host", cmd.Flags().Lookup("host")); err != nil {
		return nil, err
	}

	return v, nil
}

func serve(v *viper.Viper) error {
	router := gin.New()
	router.Use(gin.Logger(), api.Recovery())

	trustedProxies := v.GetStringSlice("trusted_proxies")
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return fmt.Errorf("SetTrustedProxies: %w", err)
	}

	api.SetupRoutes(router)

	addr := fmt.Sprintf("%s:%d", v.GetString("host"), v.GetInt("port"))

	s := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		err := s.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown:", err)
	}

	log.Println("Server exiting")

	return nil
}
