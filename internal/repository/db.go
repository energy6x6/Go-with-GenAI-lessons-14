package repository

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"libratrack/internal/config"
)

func OpenDB(ctx context.Context, c config.Config) (*gorm.DB, error) {
	sqlLogger := logger.New(log.New(os.Stdout, "SQL ", log.LstdFlags), logger.Config{
		SlowThreshold: time.Second, LogLevel: logger.Info, Colorful: false, ParameterizedQueries: true,
	})
	db, err := gorm.Open(postgres.Open(c.DatabaseURL), &gorm.Config{Logger: sqlLogger, DisableAutomaticPing: true})
	if err != nil {
		return nil, errors.New("cannot initialize database connection")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(c.MaxOpenConns)
	pool.SetMaxIdleConns(c.MaxIdleConns)
	pool.SetConnMaxLifetime(c.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(5 * time.Minute)
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, errors.New("cannot connect to PostgreSQL; check server and credentials")
	}
	return db, nil
}
