package repository_test

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

func TestConcurrentLoansCannotBorrowSameCopy(t *testing.T) {
	db := testutil.Database(t)
	var copy model.Copy
	if err := db.Where("status = ?", "available").First(&copy).Error; err != nil {
		t.Fatal(err)
	}
	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewLoanRepository(db)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := repo.Borrow(t.Context(), model.BorrowRequest{CopyID: copy.ID, UserID: user.ID}, time.Now().Add(14*24*time.Hour))
			results <- err
		}()
	}
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("double booking", success, conflict)
	}
	var count int64
	if err := db.Model(&model.Loan{}).Where("copy_id = ? AND returned_at IS NULL", copy.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("active loan count", count)
	}
}

func TestConcurrentRegistrationEnforcesCaseInsensitiveUniqueness(t *testing.T) {
	db := testutil.Database(t)
	repo := repository.NewUserRepository(db)
	var start sync.WaitGroup
	start.Add(1)
	results := make(chan error, 2)
	for _, email := range []string{"Concurrent@example.com", "concurrent@example.com"} {
		go func(email string) {
			start.Wait()
			user := model.User{Email: email, Role: model.RoleReader}
			results <- repo.Create(t.Context(), &user)
		}(email)
	}
	start.Done()
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("duplicate accounts", success, conflict)
	}
}

func TestSecurityMigrationsUpDownPreserveInheritedRole(t *testing.T) {
	db := testutil.Database(t)
	dsn := db.Dialector.(*postgres.Dialector).Config.DSN
	m, err := repository.Migration(dsn, filepath.Join(testutil.Root(), "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(2); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn("users", "role") || db.Migrator().HasColumn("users", "password_hash") || db.Migrator().HasTable("loans") {
		t.Fatal("rollback removed inherited role or left security schema")
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn("users", "password_hash") || !db.Migrator().HasTable("loans") {
		t.Fatal("security schema not restored")
	}
}
