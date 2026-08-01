package main

import (
	"context"
	"errors"
	"fmt"
	"ledger/api"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func main() {
	v := setupConfig()

	serve(v)
}

func setupConfig() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix("ledger")
	v.AutomaticEnv()

	v.SetDefault("host", "localhost")
	v.SetDefault("port", "8080")
	v.SetDefault("trusted_proxies", []string{})

	return v
}

func serve(v *viper.Viper) {
	router := gin.Default()

	trustedProxies := v.GetStringSlice("trusted_proxies")
	err := router.SetTrustedProxies(trustedProxies)

	if err != nil {
		log.Fatalf("SetTrustedProxies: %s\n", err)
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
}
