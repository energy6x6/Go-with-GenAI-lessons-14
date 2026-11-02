package repository

import (
	"context"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"libratrack/internal/model"
)

type gormBookRepository struct{ db *gorm.DB }

func NewBookRepository(db *gorm.DB) BookRepository { return &gormBookRepository{db: db} }

func references(tx *gorm.DB, input model.BookInput) error {
	if !input.Valid() {
		return ErrValidation
	}
	var category model.Category
	if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).First(&category, input.CategoryID).Error; err != nil {
		if translate(err) == ErrNotFound {
			return ErrInvalidReference
		}
		return err
	}
	var authors []model.Author
	if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).Where("id IN ?", input.AuthorIDs).Find(&authors).Error; err != nil {
		return err
	}
	if len(authors) != len(input.AuthorIDs) {
		return ErrInvalidReference
	}
	return nil
}

func replaceAuthors(tx *gorm.DB, bookID uint, ids []uint) error {
	if err := tx.Exec("DELETE FROM book_authors WHERE book_id = ?", bookID).Error; err != nil {
		return err
	}
	links := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		links = append(links, map[string]interface{}{"book_id": bookID, "author_id": id})
	}
	return tx.Table("book_authors").Create(&links).Error
}

func loadBook(db *gorm.DB, id uint) (model.Book, error) {
	var b model.Book
	err := db.Preload("Authors", func(q *gorm.DB) *gorm.DB { return q.Order("authors.id ASC") }).Preload("Category").First(&b, id).Error
	return b, translate(err)
}

func (r *gormBookRepository) Create(ctx context.Context, input model.BookInput) (model.Book, error) {
	var result model.Book
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := references(tx, input); err != nil {
			return err
		}
		b := model.Book{Title: input.Title, ISBN: input.ISBN, CategoryID: input.CategoryID, PublishedYear: input.PublishedYear, Description: input.Description}
		if err := tx.Omit(clause.Associations).Create(&b).Error; err != nil {
			return err
		}
		if err := replaceAuthors(tx, b.ID, input.AuthorIDs); err != nil {
			return err
		}
		var err error
		result, err = loadBook(tx, b.ID)
		return err
	})
	return result, translate(err)
}

func (r *gormBookRepository) FindByID(ctx context.Context, id uint) (model.Book, error) {
	return loadBook(r.db.WithContext(ctx), id)
}

func (r *gormBookRepository) Update(ctx context.Context, id uint, input model.BookInput) (model.Book, error) {
	var result model.Book
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.Book
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, id).Error; err != nil {
			return err
		}
		if err := references(tx, input); err != nil {
			return err
		}
		values := map[string]interface{}{"title": input.Title, "isbn": input.ISBN, "category_id": input.CategoryID, "published_year": input.PublishedYear, "description": input.Description, "updated_at": time.Now().UTC()}
		if err := tx.Model(&old).Updates(values).Error; err != nil {
			return err
		}
		if err := replaceAuthors(tx, id, input.AuthorIDs); err != nil {
			return err
		}
		var err error
		result, err = loadBook(tx, id)
		return err
	})
	return result, translate(err)
}

func (r *gormBookRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Book{}, id)
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gormBookRepository) List(ctx context.Context, f BookFilter) ([]model.Book, error) {
	books := make([]model.Book, 0)
	if f.Page == 0 || f.Limit == 0 {
		return books, ErrValidation
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Page-1 > uint64(math.MaxInt64)/f.Limit {
		return books, nil
	}
	q := r.db.WithContext(ctx).Order("books.id ASC").Limit(int(f.Limit)).Offset(int((f.Page - 1) * f.Limit))
	if f.IncludeAvailability {
		q = q.Select("books.*, COALESCE(stock.available_copies, 0) AS available_copies, COALESCE(stock.total_copies, 0) AS total_copies").Joins(`LEFT JOIN (SELECT book_id, COUNT(*) AS total_copies, COUNT(*) FILTER (WHERE status = 'available') AS available_copies FROM copies GROUP BY book_id) AS stock ON stock.book_id = books.id`)
	}
	if f.Search != "" {
		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.Search)
		q = q.Where("books.title ILIKE ? OR books.isbn ILIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}
	if f.CategoryID != 0 {
		q = q.Where("books.category_id = ?", f.CategoryID)
	}
	if err := q.Preload("Authors").Preload("Category").Find(&books).Error; err != nil {
		return nil, translate(err)
	}
	return books, nil
}
