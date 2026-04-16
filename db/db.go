package db

import (
	"database/sql"
	"embed"
	"errors"
	"log"

	pgxadapter "github.com/noho-digital/casbin-pgx-adapter"
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed casbin_model.conf
var casbinModel string

func InitDB(dsn string) *sql.DB {
	if dsn == "" {
		panic(errors.New("database URL is empty"))
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
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

	adapter, err := pgxadapter.NewAdapter(dsn)
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
