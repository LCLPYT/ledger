package assetrepo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/lclpyt/ledger/internal/auth"
	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
	"github.com/quadrubo/golib/logging"
	"github.com/samber/do/v2"
)

const configKey = "modules.asset_repo"

func Module() app.Module {
	return &module{}
}

type module struct {
	enabled bool
	lis     net.Listener
	server  *http.Server
	log     *slog.Logger
	cfg     Config
}

type Config struct {
	Addr        string        `config:"addr" validate:"required"`
	ConfigFile  string        `config:"config_file"`
	GracePeriod time.Duration `config:"grace_period" validate:"required"`
}

func (m *module) Name() string {
	return "asset_repo"
}

func (m *module) Provide(_ context.Context, i do.Injector) error {
	log, err := logging.Component(i, m.Name())

	if err != nil {
		return fmt.Errorf("asset_repo: failed to inject logger: %w", err)
	}

	m.log = log

	m.cfg, err = config.Load(i, m.Name(), configKey, Config{
		Addr:        ":8080",
		ConfigFile:  "",
		GracePeriod: 5 * time.Second,
	})

	if err != nil {
		return fmt.Errorf("asset_repo: failed to load config: %w", err)
	}

	if m.cfg.ConfigFile == "" {
		m.enabled = false
		log.Info("asset_repo: no config was given, disabling feature")
		return nil
	}

	cfg, err := ReadConfig(m.cfg.ConfigFile)

	if err != nil {
		return err
	}

	log.Debug("asset repo config read successfully", slog.Int("repositories", len(cfg.Repos)))

	authenticator, err := do.Invoke[*auth.Authenticator](i)

	if err != nil {
		return fmt.Errorf("asset_repo: failed to inject the authenticator: %w", err)
	}

	mux := http.NewServeMux()

	rootRoute := "/v1/assets"

	for _, repo := range cfg.Repos {
		fsys, err := repo.FileSystem()

		if err != nil {
			return fmt.Errorf("asset_repo: failed to serve asset repo %s: %w", repo.Id, err)
		}

		prefix := fmt.Sprintf("%s/%s/", rootRoute, repo.Id)

		files := http.StripPrefix(prefix, http.FileServer(fsys))

		if !repo.Public {
			files = auth.RequireHTTP(authenticator, log, repo.ReadPermission(), files)
		}

		mux.Handle(fmt.Sprintf("GET %s", prefix), files)

		log.Info("serving asset repo", slog.String("id", repo.Id), slog.String("path", repo.Path),
			slog.Bool("public", repo.Public))
	}

	handler := m.withLogging(mux)

	budget, err := do.Invoke[app.DrainBudget](i)

	if err != nil {
		return fmt.Errorf("asset_repo: failed to inject the drain budget: %w", err)
	}

	if m.cfg.GracePeriod >= time.Duration(budget) {
		return fmt.Errorf("asset_repo: grace_period of %s has to be less than the %s drain budget",
			m.cfg.GracePeriod, budget)
	}

	m.enabled = true

	m.lis, err = net.Listen("tcp", m.cfg.Addr)

	if err != nil {
		return fmt.Errorf("asset_repo: failed to listen on %s: %w", m.cfg.Addr, err)
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
	if !m.enabled {
		return nil
	}

	_ = m.server.Close()
	_ = m.lis.Close()

	return nil
}

func (m *module) Run(ctx context.Context) error {
	if !m.enabled {
		<-ctx.Done()
		return nil
	}

	m.log.Info("serving asset repos", slog.String("addr", m.lis.Addr().String()))

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- m.server.Serve(m.lis)
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("asset_repo: failed to serve: %w", err)
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
		m.log.Warn("shutdown completed with an error, dropping still active requests", slog.Any("error", err))
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
