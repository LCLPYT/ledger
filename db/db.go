package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func ConnectDatabase(v *viper.Viper) *bun.DB {
	db := connectPostgres(v)

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	return db
}

func connectPostgres(v *viper.Viper) *bun.DB {
	v.SetDefault("database_port", 5432)

	user := v.GetString("database_user")
	password := v.GetString("database_password")
	host := v.GetString("database_host")
	port := v.GetInt("database_port")
	database := v.GetString("database_name")

	if user == "" {
		panic("postgres database user not specified")
	}

	if password == "" {
		panic("postgres database password not specified")
	}

	if host == "" {
		panic("postgres database host not specified")
	}

	if database == "" {
		panic("postgres database name not specified")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, password, host, port, database)

	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
	))

	db := bun.NewDB(sqldb, pgdialect.New())

	return db
}
