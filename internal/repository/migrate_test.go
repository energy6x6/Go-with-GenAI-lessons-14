package repository_test

import (
	"path/filepath"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
)

func TestMigrationsUpDownUpPreserveLegacyData(t *testing.T) {
	db := testutil.Database(t)
	dsn := db.Dialector.(*postgres.Dialector).Config.DSN
	m, err := repository.Migration(dsn, filepath.Join(testutil.Root(), "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	// Remove demo data and normalization, retaining the initial flat schema.
	if err := m.Migrate(1); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO books(title,isbn,author,category,published_year,created_at)
 VALUES('Legacy Book','legacy-roundtrip','Original Author','Original Category',0,'2010-01-01T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	var b model.Book
	if err := db.Preload("Authors").Preload("Category").Where("isbn = ?", "legacy-roundtrip").First(&b).Error; err != nil {
		t.Fatal(err)
	}
	if len(b.Authors) != 1 || b.Authors[0].FullName != "Original Author" || b.Category.Name != "Original Category" || b.PublishedYear == nil || *b.PublishedYear != 0 || b.CreatedAt.Year() != 2010 {
		t.Fatalf("normalization changed legacy data: %+v", b)
	}
	if err := m.Migrate(1); err != nil {
		t.Fatal(err)
	}
	var flat struct{ Author, Category string }
	if err := db.Table("books").Select("author,category").Where("isbn = ?", "legacy-roundtrip").Scan(&flat).Error; err != nil {
		t.Fatal(err)
	}
	if flat.Author != "Original Author" || flat.Category != "Original Category" {
		t.Fatal(flat)
	}
	if err := m.Down(); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Book{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Fatal("unexpected seed count", count)
	}
}

func TestConcurrentDuplicateCreateIsAtomic(t *testing.T) {
	db := testutil.Database(t)
	r := repository.NewBookRepository(db)
	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < 2; i++ {
		go func() { start.Wait(); _, err := r.Create(ctx, input("test-concurrent-isbn")); results <- err }()
	}
	start.Done()
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if err == repository.ErrConflict {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
	var count int64
	if err := db.Table("book_authors").Joins("JOIN books ON books.id=book_authors.book_id").Where("books.isbn = ?", "test-concurrent-isbn").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("partial or duplicate author links", count)
	}
}

func TestEmbeddedMigrationsFromEmptyWorkingDirectory(t *testing.T) {
	db := testutil.Database(t)
	dsn := db.Dialector.(*postgres.Dialector).Config.DSN
	t.Chdir(t.TempDir())
	m, err := repository.Migration(dsn, "")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Down(); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	version, dirty, err := m.Version()
	if err != nil || dirty || version != 5 {
		t.Fatalf("version=%d dirty=%t err=%v", version, dirty, err)
	}
	var count int64
	if err := db.Model(&model.Book{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Fatalf("embedded seed contains %d books", count)
	}
}
