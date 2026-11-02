package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL     string
	Port            string
	ListenHost      string
	MigrationsPath  string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Load keeps existing environment values authoritative, including in CI.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, errors.New("cannot read .env configuration")
	}
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Port: env("PORT", "8085"), ListenHost: env("LISTEN_HOST", "127.0.0.1"), MigrationsPath: os.Getenv("MIGRATIONS_PATH")}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return c, errors.New("PORT must be between 1 and 65535")
	}
	if c.DatabaseURL == "" {
		user, password := os.Getenv("APP_DB_USER"), os.Getenv("APP_DB_PASSWORD")
		if user == "" || password == "" {
			return c, errors.New("set DATABASE_URL or APP_DB_USER and APP_DB_PASSWORD")
		}
		u := url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(env("POSTGRES_HOST", "127.0.0.1"), env("POSTGRES_PORT", "5432")), Path: "/" + env("POSTGRES_DB", "libratrack")}
		q := u.Query()
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		c.DatabaseURL = u.String()
	}
	if err := ValidateDatabaseURL(c.DatabaseURL); err != nil {
		return c, err
	}
	c.MaxOpenConns, err = positiveInt("DB_MAX_OPEN_CONNS", 10)
	if err != nil {
		return c, err
	}
	c.MaxIdleConns, err = positiveInt("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return c, err
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return c, errors.New("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	c.ConnMaxLifetime, err = time.ParseDuration(env("DB_CONN_MAX_LIFETIME", "30m"))
	if err != nil || c.ConnMaxLifetime <= 0 {
		return c, errors.New("DB_CONN_MAX_LIFETIME must be a positive duration")
	}
	return c, nil
}

func ValidateDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/" {
		return errors.New("database URL must name a PostgreSQL host and database")
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func positiveInt(key string, fallback int) (int, error) {
	n, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}
