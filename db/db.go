package db

import (
	"database/sql"
	"errors"
	"log"

	casbinpg "github.com/casbin/casbin-pg-adapter"
	"github.com/casbin/casbin/v2"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

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

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		panic(err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("Migration failed: %v", err)
	}

	return db
}

func InitCasbin(dsn string, modelPath string) *casbin.Enforcer {
	adapter, err := casbinpg.NewAdapter(dsn)
	if err != nil {
		panic(err)
	}

	enforcer, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		panic(err)
	}

	err = enforcer.LoadPolicy()
	if err != nil {
		panic(err)
	}

	return enforcer
}
