package repository_test

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/internal/testutil"
	"math"
	"testing"
)

var ctx = context.Background()

func input(isbn string) model.BookInput {
	return model.BookInput{Title: "Test Book", ISBN: isbn, CategoryID: 1, AuthorIDs: []uint{1, 2}}
}
func create(t *testing.T, r repository.BookRepository, isbn string) model.Book {
	t.Helper()
	b, err := r.Create(ctx, input(isbn))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBookCreateLoadsRelations(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	b := create(t, r, "test-create")
	if b.ID == 0 || len(b.Authors) != 2 || b.Category.ID != 1 || b.CreatedAt.IsZero() {
		t.Fatalf("incomplete book: %+v", b)
	}
}
func TestBookDuplicateISBN(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	create(t, r, "test-duplicate")
	_, err := r.Create(ctx, input("test-duplicate"))
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatal(err)
	}
}
func TestBookUnknownAuthorDoesNotCreateData(t *testing.T) {
	db := testutil.Database(t)
	r := repository.NewBookRepository(db)
	in := input("test-author")
	in.AuthorIDs = []uint{999999}
	_, err := r.Create(ctx, in)
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.Book{}).Where("isbn = ?", in.ISBN).Count(&count)
	if count != 0 {
		t.Fatal("failed create left a book")
	}
	db.Model(&model.Author{}).Where("id = ?", 999999).Count(&count)
	if count != 0 {
		t.Fatal("unknown author was created")
	}
}
func TestBookUnknownCategory(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	in := input("test-category")
	in.CategoryID = 999999
	_, err := r.Create(ctx, in)
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatal(err)
	}
}
func TestBookFindNotFound(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	_, err := r.FindByID(ctx, 999999)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestBookUpdateReplacesAuthors(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	b := create(t, r, "test-update")
	in := input(b.ISBN)
	in.AuthorIDs = []uint{3}
	in.Title = "Replaced"
	updated, err := r.Update(ctx, b.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != in.Title || len(updated.Authors) != 1 || updated.Authors[0].ID != 3 || !updated.CreatedAt.Equal(b.CreatedAt) {
		t.Fatalf("incorrect update: %+v", updated)
	}
}
func TestBookUpdateClearsNullableFields(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	year := 0
	description := ""
	in := input("test-null")
	in.PublishedYear = &year
	in.Description = &description
	b, err := r.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if b.PublishedYear == nil || *b.PublishedYear != 0 {
		t.Fatal("explicit zero lost")
	}
	updated, err := r.Update(ctx, b.ID, input(in.ISBN))
	if err != nil {
		t.Fatal(err)
	}
	if updated.PublishedYear != nil || updated.Description != nil {
		t.Fatal("omitted optionals were not cleared")
	}
}
func TestBookFailedUpdateRollsBack(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	b := create(t, r, "test-rollback")
	in := input(b.ISBN)
	in.Title = "Should not survive"
	in.AuthorIDs = []uint{999999}
	_, err := r.Update(ctx, b.ID, in)
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatal(err)
	}
	actual, err := r.FindByID(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Title != b.Title || len(actual.Authors) != 2 {
		t.Fatal("partial update survived")
	}
}
func TestBookUpdateConflictRollsBack(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	a := create(t, r, "test-conflict-a")
	b := create(t, r, "test-conflict-b")
	in := input(a.ISBN)
	in.AuthorIDs = []uint{3}
	_, err := r.Update(ctx, b.ID, in)
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatal(err)
	}
	actual, err := r.FindByID(ctx, b.ID)
	if err != nil || actual.ISBN != b.ISBN || len(actual.Authors) != 2 {
		t.Fatal("conflicting update changed data")
	}
}
func TestBookDeleteCascades(t *testing.T) {
	db := testutil.Database(t)
	r := repository.NewBookRepository(db)
	b := create(t, r, "test-cascade")
	copies := repository.NewCopyRepository(db)
	if _, err := copies.Create(ctx, b.ID, model.CopyInput{InventoryNo: "cascade-1"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"copies", "book_authors"} {
		var count int64
		if err := db.Table(table).Where("book_id = ?", b.ID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal(table, "was not cascaded")
		}
	}
	if _, err := repository.NewAuthorRepository(db).FindByID(ctx, 1); err != nil {
		t.Fatal("deleting book removed author")
	}
}
func TestBookDeleteNotFound(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	if !errors.Is(r.Delete(ctx, 999999), repository.ErrNotFound) {
		t.Fatal("missing book deletion succeeded")
	}
}
func TestBookListFilterAndPagination(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	books, err := r.List(ctx, repository.BookFilter{CategoryID: 1, Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 || books[0].CategoryID != 1 || books[0].ID >= books[1].ID || len(books[0].Authors) == 0 {
		t.Fatalf("bad list: %+v", books)
	}
}
func TestBookHugePageReturnsEmpty(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	books, err := r.List(ctx, repository.BookFilter{Page: math.MaxUint64, Limit: 100})
	if err != nil || books == nil || len(books) != 0 {
		t.Fatal("huge page did not return []", err)
	}
}
func TestBookCancelledContext(t *testing.T) {
	r := repository.NewBookRepository(testutil.Database(t))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := r.FindByID(cancelled, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestBookListConstantQueries(t *testing.T) {
	db := testutil.Database(t)
	db, count := testutil.CountQueries(db)
	r := repository.NewBookRepository(db)
	_, err := r.List(ctx, repository.BookFilter{Page: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	first := count.Load()
	for i := 0; i < 5; i++ {
		in := input("scale-" + string(rune('a'+i)))
		if _, err := r.Create(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	count.Store(0)
	_, err = r.List(ctx, repository.BookFilter{Page: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	second := count.Load()
	t.Logf("list SELECTs before scaling=%d after=%d", first, second)
	if first < 1 || first > 6 || first != second {
		t.Fatalf("N+1 detected: before=%d after=%d", first, second)
	}
}

func assertRejected(t *testing.T, db *gorm.DB, sql string) {
	t.Helper()
	if err := db.Exec(sql).Error; err == nil {
		t.Fatal("invalid SQL data accepted")
	}
}
func TestDatabaseRejectsInvalidStatus(t *testing.T) {
	db := testutil.Database(t)
	assertRejected(t, db, "INSERT INTO copies(book_id,inventory_no,status) VALUES(1,'bad-status','broken')")
}
func TestDatabaseRejectsInvalidRole(t *testing.T) {
	db := testutil.Database(t)
	assertRejected(t, db, "INSERT INTO users(email,role) VALUES('bad-role@example.com','boss')")
}
func TestDatabaseRestrictsReferencedAuthor(t *testing.T) {
	db := testutil.Database(t)
	assertRejected(t, db, "DELETE FROM authors WHERE id=1")
}
func TestDatabaseRestrictsReferencedCategory(t *testing.T) {
	db := testutil.Database(t)
	assertRejected(t, db, "DELETE FROM categories WHERE id=1")
}
func TestCategoryNameConflict(t *testing.T) {
	r := repository.NewCategoryRepository(testutil.Database(t))
	_, err := r.Create(ctx, model.CategoryInput{Name: "Programming"})
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatal(err)
	}
}
func TestAuthorCreateAndFind(t *testing.T) {
	r := repository.NewAuthorRepository(testutil.Database(t))
	a, err := r.Create(ctx, model.AuthorInput{FullName: "Test Author"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.FindByID(ctx, a.ID)
	if err != nil || got.FullName != a.FullName {
		t.Fatal(got, err)
	}
}
func TestCopyDefaultAndAvailability(t *testing.T) {
	db := testutil.Database(t)
	b := create(t, repository.NewBookRepository(db), "test-copies")
	r := repository.NewCopyRepository(db)
	c, err := r.Create(ctx, b.ID, model.CopyInput{InventoryNo: "copy-1"})
	if err != nil || c.Status != "available" {
		t.Fatal(c, err)
	}
	if _, err := r.Create(ctx, b.ID, model.CopyInput{InventoryNo: "copy-2", Status: "lost"}); err != nil {
		t.Fatal(err)
	}
	a, err := r.Availability(ctx, b.ID)
	if err != nil || a.TotalCopies != 2 || a.AvailableCopies != 1 || a.BookID != b.ID {
		t.Fatal(a, err)
	}
}
func TestCopyDuplicateInventory(t *testing.T) {
	r := repository.NewCopyRepository(testutil.Database(t))
	in := model.CopyInput{InventoryNo: "duplicate-copy"}
	if _, err := r.Create(ctx, 1, in); err != nil {
		t.Fatal(err)
	}
	_, err := r.Create(ctx, 2, in)
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatal(err)
	}
}
func TestCopiesUnknownBook(t *testing.T) {
	r := repository.NewCopyRepository(testutil.Database(t))
	if _, err := r.List(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, 999999, model.CopyInput{InventoryNo: "missing-book"}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := r.Availability(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestBookWithoutCopiesHasZeroAvailability(t *testing.T) {
	db := testutil.Database(t)
	b := create(t, repository.NewBookRepository(db), "test-empty-copies")
	a, err := repository.NewCopyRepository(db).Availability(ctx, b.ID)
	if err != nil || a.TotalCopies != 0 || a.AvailableCopies != 0 || a.BookID != b.ID {
		t.Fatal(a, err)
	}
}
