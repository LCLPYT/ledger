package maprepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
	"github.com/quadrubo/golib/logging"
	"github.com/samber/do/v2"
)

const configKey = "modules.map_repo"

func Module() app.Module {
	return &module{}
}

type module struct {
	lis    net.Listener
	server *http.Server
	log    *slog.Logger
	cfg    Config
}

type Config struct {
	Addr        string        `config:"addr" validate:"required"`
	RootDir     string        `config:"root_dir" validate:"required"`
	GracePeriod time.Duration `config:"grace_period" validate:"required"`
}

func (m *module) Name() string {
	return "map_repo"
}

func (m *module) Provide(_ context.Context, i do.Injector) error {
	cfg, err := config.Load(i, m.Name(), configKey, Config{
		Addr:        ":8080",
		RootDir:     "./data/maps",
		GracePeriod: 5 * time.Second,
	})

	if err != nil {
		return fmt.Errorf("map_repo: failed to load config: %w", err)
	}

	m.cfg = cfg

	if info, err := os.Stat(cfg.RootDir); err != nil {
		return fmt.Errorf("map_repo: failed to access root_dir: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("map_repo: root_dir %s is not a directory", cfg.RootDir)
	}

	m.log, err = logging.Component(i, m.Name())

	if err != nil {
		return fmt.Errorf("map_repo: failed to inject logger: %w", err)
	}

	mux := http.NewServeMux()

	mux.Handle("GET /v1/maps/", http.StripPrefix("/v1/maps/", http.FileServer(http.Dir(cfg.RootDir))))

	handler := m.withLogging(mux)

	budget, err := do.Invoke[app.DrainBudget](i)

	if err != nil {
		return fmt.Errorf("map_repo: failed to inject the drain budget: %w", err)
	}

	if cfg.GracePeriod >= time.Duration(budget) {
		return fmt.Errorf("map_repo: grace_period of %s has to be less than the %s drain budget",
			cfg.GracePeriod, budget)
	}

	m.lis, err = net.Listen("tcp", cfg.Addr)

	if err != nil {
		return fmt.Errorf("map_repo: failed to listen on %s: %w", cfg.Addr, err)
	}

	m.server = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 200 * time.Millisecond,
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return nil
}

func (m *module) Stop(context.Context) error {
	_ = m.server.Close()
	_ = m.lis.Close()

	return nil
}

func (m *module) Run(ctx context.Context) error {
	m.log.Info("serving map repo", slog.String("addr", m.lis.Addr().String()),
		slog.String("root_dir", m.cfg.RootDir))

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- m.server.Serve(m.lis)
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("map_repo: failed to serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	m.stop()

	return nil
}

// stop lets in-flight requests finish within the grace period, then drops them.
func (m *module) stop() {
	m.log.Info("stopping", slog.Duration("grace_period", m.cfg.GracePeriod))

	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.GracePeriod)
	defer cancel()

	if err := m.server.Shutdown(ctx); err != nil {
		m.log.Warn("shutdown completed with an error: %w, dropping still active requests", err)
		_ = m.server.Close()
		return
	}

	m.log.Info("stopped")
}

func (m *module) withLogging(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		m.log.Info("request",
			slog.String("remote", r.RemoteAddr),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)))
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// ReadFrom keeps the sendfile fast path of the underlying writer for file transfers.
func (r *statusRecorder) ReadFrom(src io.Reader) (int64, error) {
	return io.Copy(r.ResponseWriter, src)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
