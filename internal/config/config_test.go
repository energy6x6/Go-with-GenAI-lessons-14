package config

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func defaults(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, key := range []string{"DATABASE_URL", "APP_DB_USER", "APP_DB_PASSWORD", "PORT", "MIGRATIONS_PATH", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB"} {
		t.Setenv(key, "")
	}
}
func TestBuildURLQuotesCredentials(t *testing.T) {
	defaults(t)
	t.Setenv("APP_DB_USER", "user@example")
	t.Setenv("APP_DB_PASSWORD", "p@ss:/?#%")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := u.User.Password()
	if u.User.Username() != "user@example" || password != "p@ss:/?#%" {
		t.Fatal("credentials changed during encoding")
	}
}
func TestEnvironmentOverridesDotenv(t *testing.T) {
	defaults(t)
	t.Setenv("DATABASE_URL", "postgres://ci:ci@localhost/ci_test?sslmode=disable")
	if err := os.WriteFile(filepath.Join(".", ".env"), []byte("DATABASE_URL=postgres://local:local@localhost/local\nPORT=8099\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "postgres://ci:ci@localhost/ci_test?sslmode=disable" {
		t.Fatal("CI URL overwritten")
	}
}
func TestMissingCredentialsFails(t *testing.T) {
	defaults(t)
	if _, err := Load(); err == nil {
		t.Fatal("missing credentials accepted")
	}
}
func TestInvalidPoolFails(t *testing.T) {
	defaults(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("DB_MAX_OPEN_CONNS", "2")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	if _, err := Load(); err == nil {
		t.Fatal("invalid pool accepted")
	}
}
func TestInvalidPortFails(t *testing.T) {
	defaults(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	t.Setenv("PORT", "99999")
	if _, err := Load(); err == nil {
		t.Fatal("invalid port accepted")
	}
}
func TestMalformedDotenvFails(t *testing.T) {
	defaults(t)
	if err := os.WriteFile(".env", []byte("BROKEN='unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("malformed .env accepted")
	}
}
