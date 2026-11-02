package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"libratrack/internal/app"
	"libratrack/internal/auth"
	"libratrack/internal/config"
	"libratrack/internal/repository"
)

// @title LibraTrack Library API
// @version 4.0
// @description Lesson 10: HTML templates, HTMX, cookie authentication and embedded assets.
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter Bearer followed by your access token.
func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	tokens, err := auth.NewTokensFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := repository.OpenDB(connectCtx, c)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		log.Fatal("cannot access database pool")
	}
	defer pool.Close()
	if err := repository.Migrate(c.DatabaseURL, c.MigrationsPath); err != nil {
		log.Fatalf("cannot apply SQL migrations (%T)", err)
	}
	router, err := app.NewRouter(repository.NewRepositories(db), tokens)
	if err != nil {
		log.Fatalf("cannot initialize application: %v", err)
	}
	server := &http.Server{Addr: net.JoinHostPort(c.ListenHost, c.Port), Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-signals.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown failed (%T)", err)
		}
	}()
	log.Printf("listening on http://localhost:%s/catalog; Swagger: /swagger/index.html", c.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("HTTP server failed")
	}
}
