package wiring

import (
	"ledger/internal/bob"
	"ledger/internal/db/migrations"

	"github.com/quadrubo/golib/app"
	"github.com/quadrubo/golib/config"
	"github.com/quadrubo/golib/logging"
	"github.com/quadrubo/golib/migrate"
	"github.com/quadrubo/golib/postgres"
)

const envPrefix = "LEDGER__"

// Config is the settings module every other module resolves. Tests pass
// config.StaticModule to the sets below instead.
func Config() app.Module {
	return config.Module(config.Options{EnvPrefix: envPrefix})
}

func Base(conf app.Module) []app.Module {
	return []app.Module{conf, logging.Module()}
}

func Database(conf app.Module) []app.Module {
	return append(
		Base(conf),
		postgres.Module(),
		bob.Module(),
	)
}

// Migrate applies the pending migrations while the app boots.
func Migrate(conf app.Module) []app.Module {
	return append(Database(conf), migrate.Module(migrations.FS))
}

func Serve(conf app.Module) []app.Module {
	return Migrate(conf)
}
