package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
	"github.com/samber/do/v2"
	"github.com/stephenafamo/bob"
)

const configKey = "modules.auth"

// Module provides the *Authenticator the gRPC interceptor and the asset repos use.
func Module() app.Module {
	return &module{}
}

type module struct{}

var _ app.Provider = (*module)(nil)

type Config struct {
	CacheTTL time.Duration `config:"cache_ttl" validate:"min=0"`
}

func (m *module) Name() string {
	return "auth"
}

func (m *module) Provide(_ context.Context, i do.Injector) error {
	cfg, err := config.Load(i, m.Name(), configKey, Config{
		CacheTTL: 30 * time.Second,
	})

	if err != nil {
		return fmt.Errorf("auth: failed to load config: %w", err)
	}

	db, err := do.Invoke[bob.DB](i)

	if err != nil {
		return fmt.Errorf("auth: failed to inject the db: %w", err)
	}

	do.ProvideValue(i, NewAuthenticator(db, cfg.CacheTTL))

	return nil
}
