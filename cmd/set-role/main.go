// Local administrative command: requires database credentials, never exposed as a public route.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"libratrack/internal/config"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

func main() {
	email := flag.String("email", "", "Email of an existing registered account")
	role := flag.String("role", "", "reader, librarian or admin")
	flag.Parse()
	if *email == "" || !model.ValidRole(*role) {
		log.Fatal("provide -email and -role reader|librarian|admin")
	}
	c, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := repository.OpenDB(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		log.Fatal("cannot access database pool")
	}
	defer pool.Close()
	repo := repository.NewUserRepository(db)
	user, err := repo.FindByEmail(ctx, *email)
	if err != nil {
		log.Fatal("registered account not found or database unavailable")
	}
	if _, err := repo.SetRole(ctx, user.ID, *role); err != nil {
		log.Fatal("cannot set account role")
	}
	log.Printf("Updated user_id=%d role=%s; log in again to obtain a new token", user.ID, *role)
}
