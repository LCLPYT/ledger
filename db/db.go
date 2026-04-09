package db

import (
	"database/sql"
	"embed"
	"errors"
	"log"

	casbinpg "github.com/casbin/casbin-pg-adapter"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed casbin_model.conf
var casbinModel string

func InitDB(dsn string) *sql.DB {
	if dsn == "" {
		panic(errors.New("database URL is empty"))
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		panic(err)
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		panic(err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		panic(err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("Migration failed: %v", err)
	}

	return db
}

func InitCasbin(dsn string) *casbin.Enforcer {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		panic(err)
	}

	adapter, err := casbinpg.NewAdapter(dsn)
	if err != nil {
		panic(err)
	}

	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		panic(err)
	}

	if err := enforcer.LoadPolicy(); err != nil {
		panic(err)
	}

	return enforcer
}
