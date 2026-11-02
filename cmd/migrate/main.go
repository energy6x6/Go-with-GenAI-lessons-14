package main

import (
	"errors"
	"github.com/golang-migrate/migrate/v4"
	"libratrack/internal/config"
	"libratrack/internal/repository"
	"log"
	"os"
	"strconv"
)

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	m, err := repository.Migration(c.DatabaseURL, c.MigrationsPath)
	if err != nil {
		log.Fatal("cannot initialize migrations")
	}
	defer m.Close()
	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	switch action {
	case "up":
		err = m.Up()
	case "down":
		if len(os.Args) != 3 {
			log.Fatal("usage: go run ./cmd/migrate down <positive-step-count>")
		}
		n, e := strconv.Atoi(os.Args[2])
		if e != nil || n < 1 {
			log.Fatal("step count must be positive")
		}
		err = m.Steps(-n)
	case "version":
		v, dirty, e := m.Version()
		err = e
		if e == nil {
			log.Printf("version=%d dirty=%t", v, dirty)
		}
	default:
		log.Fatal("usage: go run ./cmd/migrate [up|version|down <steps>]")
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration failed (%T)", err)
	}
}
