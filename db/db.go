package db

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
	"github.com/stephenafamo/bob"
)

// DSN builds the postgres connection string from configuration.
func DSN(v *viper.Viper) (string, error) {
	v.SetDefault("database_port", 5432)

	user := v.GetString("database_user")
	password := v.GetString("database_password")
	host := v.GetString("database_host")
	port := v.GetInt("database_port")
	database := v.GetString("database_name")

	if user == "" {
		return "", fmt.Errorf("postgres database user not specified")
	}

	if password == "" {
		return "", fmt.Errorf("postgres database password not specified")
	}

	if host == "" {
		return "", fmt.Errorf("postgres database host not specified")
	}

	if database == "" {
		return "", fmt.Errorf("postgres database name not specified")
	}

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}

	return dsn.String(), nil
}

// OpenSQL returns the underlying database handle used by migrations.
func OpenSQL(v *viper.Viper) (*sql.DB, error) {
	dsn, err := DSN(v)

	if err != nil {
		return nil, err
	}

	sqldb, err := sql.Open("pgx", dsn)

	if err != nil {
		return nil, err
	}

	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(25)
	sqldb.SetConnMaxLifetime(5 * time.Minute)

	return sqldb, nil
}

func ConnectDatabase(v *viper.Viper) (bob.DB, error) {
	sqldb, err := OpenSQL(v)

	if err != nil {
		return bob.DB{}, err
	}

	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return bob.DB{}, fmt.Errorf("ping database: %w", err)
	}

	return bob.NewDB(sqldb), nil
}
