// Command demo-users creates local demonstration accounts without fixed passwords.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"libratrack/internal/auth"
	"libratrack/internal/config"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

func main() {
	if _, err := os.Stat(".demo-accounts.txt"); err == nil {
		log.Print(".demo-accounts.txt already exists; kept unchanged")
		return
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := repository.OpenDB(ctx, cfg)
	if err != nil {
		log.Fatal("cannot connect to database")
	}
	pool, _ := db.DB()
	defer pool.Close()
	if err := repository.Migrate(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatal("cannot apply migrations")
	}
	users := repository.NewUserRepository(db)
	text := "Lesson 10 — local demo accounts. Keep this file private.\n\n"
	for _, role := range []string{model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
		email := role + "10@example.com"
		if _, err := users.FindByEmail(ctx, email); err == nil {
			log.Fatal("demo account already exists; existing passwords were not changed")
		} else if err != repository.ErrNotFound {
			log.Fatal("cannot check demo accounts")
		}
	}
	err = db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		users = repository.NewUserRepository(txDB)
		for _, role := range []string{model.RoleReader, model.RoleLibrarian, model.RoleAdmin} {
			var random [12]byte
			if _, err := rand.Read(random[:]); err != nil {
				return err
			}
			password := hex.EncodeToString(random[:])
			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			name := "Demo " + role
			user := model.User{Email: role + "10@example.com", Name: &name, Role: role, PasswordHash: hash}
			if err := users.Create(ctx, &user); err != nil {
				return err
			}
			text += fmt.Sprintf("Role: %s\nEmail: %s\nPassword: %s\n\n", role, user.Email, password)
		}
		file, err := os.OpenFile(".demo-accounts.txt", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		if _, err := file.WriteString(text); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	})
	if err != nil {
		log.Fatal("could not create demo accounts; inspect database and private credential file before retrying")
	}
	log.Print("Created three demo accounts; credentials saved privately in .demo-accounts.txt")
}
