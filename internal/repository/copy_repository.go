package repository

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"libratrack/internal/model"
	"strings"
)

type CopyRepository interface {
	Create(context.Context, uint, model.CopyInput) (model.Copy, error)
	List(context.Context, uint) ([]model.Copy, error)
	Availability(context.Context, uint) (model.Availability, error)
}
type copyRepository struct{ db *gorm.DB }

func NewCopyRepository(db *gorm.DB) CopyRepository { return &copyRepository{db} }
func (r *copyRepository) Create(ctx context.Context, id uint, in model.CopyInput) (model.Copy, error) {
	if in.Status == "" {
		in.Status = "available"
	}
	c := model.Copy{BookID: id, InventoryNo: in.InventoryNo, Status: in.Status}
	if strings.TrimSpace(in.InventoryNo) == "" || !model.ValidCopyStatus(in.Status) {
		return c, ErrValidation
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var b model.Book
		if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).First(&b, id).Error; err != nil {
			return err
		}
		return tx.Create(&c).Error
	})
	return c, translate(err)
}
func (r *copyRepository) List(ctx context.Context, id uint) ([]model.Copy, error) {
	c := make([]model.Copy, 0)
	var b model.Book
	if err := r.db.WithContext(ctx).First(&b, id).Error; err != nil {
		return nil, translate(err)
	}
	err := r.db.WithContext(ctx).Where("book_id = ?", id).Order("id ASC").Find(&c).Error
	return c, translate(err)
}
func (r *copyRepository) Availability(ctx context.Context, id uint) (model.Availability, error) {
	a := model.Availability{BookID: id}
	var b model.Book
	if err := r.db.WithContext(ctx).First(&b, id).Error; err != nil {
		return a, translate(err)
	}
	err := r.db.WithContext(ctx).Model(&model.Copy{}).Select("COUNT(*) AS total_copies, COUNT(*) FILTER (WHERE status = 'available') AS available_copies").Where("book_id = ?", id).Scan(&a).Error
	a.BookID = id
	return a, translate(err)
}

type Repositories struct {
	Users      UserRepository
	Loans      LoanRepository
	Books      BookRepository
	Authors    AuthorRepository
	Categories CategoryRepository
	Copies     CopyRepository
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{Users: NewUserRepository(db), Loans: NewLoanRepository(db), Books: NewBookRepository(db), Authors: NewAuthorRepository(db), Categories: NewCategoryRepository(db), Copies: NewCopyRepository(db)}
}
