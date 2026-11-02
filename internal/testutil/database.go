package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"libratrack/internal/config"
	"libratrack/internal/repository"
)

func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

// Each test owns a random schema in the dedicated test database.
func Database(t *testing.T) *gorm.DB {
	t.Helper()
	if err := godotenv.Load(filepath.Join(Root(), ".env")); err != nil && !os.IsNotExist(err) {
		t.Fatal("invalid local .env")
	}
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	if err := config.ValidateDatabaseURL(raw); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid test database URL")
	}
	if !strings.HasSuffix(strings.TrimPrefix(u.Path, "/"), "_test") {
		t.Fatal("TEST_DATABASE_URL database name must end with _test")
	}
	c := config.Config{DatabaseURL: raw, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := repository.OpenDB(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	control, _ := admin.DB()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		_ = control.Close()
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(random[:])
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = control.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		_ = control.Close()
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	c.DatabaseURL = u.String()
	db, err := repository.OpenDB(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	if err := repository.Migrate(c.DatabaseURL, filepath.Join(Root(), "migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}

type queryLogger struct {
	logger.Interface
	count *atomic.Int64
}

func (l queryLogger) Trace(ctx context.Context, start time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		l.count.Add(1)
	}
	l.Interface.Trace(ctx, start, func() (string, int64) { return sql, rows }, err)
}
func CountQueries(db *gorm.DB) (*gorm.DB, *atomic.Int64) {
	count := &atomic.Int64{}
	return db.Session(&gorm.Session{Logger: queryLogger{Interface: db.Logger, count: count}}), count
}
