package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func ConnectDatabase(v *viper.Viper) (*bun.DB, error) {
	db, err := connectPostgres(v)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func connectPostgres(v *viper.Viper) (*bun.DB, error) {
	v.SetDefault("database_port", 5432)

	user := v.GetString("database_user")
	password := v.GetString("database_password")
	host := v.GetString("database_host")
	port := v.GetInt("database_port")
	database := v.GetString("database_name")

	if user == "" {
		return nil, fmt.Errorf("postgres database user not specified")
	}

	if password == "" {
		return nil, fmt.Errorf("postgres database password not specified")
	}

	if host == "" {
		return nil, fmt.Errorf("postgres database host not specified")
	}

	if database == "" {
		return nil, fmt.Errorf("postgres database name not specified")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, password, host, port, database)

	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
	))

	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(25)
	sqldb.SetConnMaxLifetime(5 * time.Minute)

	db := bun.NewDB(sqldb, pgdialect.New())

	return db, nil
}
