package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func ConfigureDatabaseDefaults(v *viper.Viper) {
	v.SetDefault("database_type", "sqlite")
	v.SetDefault("database_file", filepath.Join(xdg.DataHome, "ledger", "ledger.sqlite"))
}

func ConnectDatabase(v *viper.Viper) *bun.DB {
	dbType := v.GetString("database_type")

	var db *bun.DB

	switch dbType {
	case "postgres":
		db = connectPostgres(v)
	case "sqlite":
		db = connectSqlite(v)
	default:
		log.Panicf("Unknown database type: %s\n", dbType)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	return db
}

func connectSqlite(v *viper.Viper) *bun.DB {
	file := v.GetString("database_file")

	if file == "" {
		panic("sqlite database file not specified")
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		panic(err)
	}

	connectString := fmt.Sprintf("file:%s?cache=shared&mode=rwc", file)
	sqldb, err := sql.Open(sqliteshim.ShimName, connectString)

	if err != nil {
		panic(err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	return db
}

func connectPostgres(v *viper.Viper) *bun.DB {
	v.SetDefault("database_port", 5432)

	user := v.GetString("database_user")
	password := v.GetString("password")
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
