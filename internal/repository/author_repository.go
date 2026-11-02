package repository

import (
	"context"
	"gorm.io/gorm"
	"libratrack/internal/model"
	"strings"
)

type AuthorRepository interface {
	Create(context.Context, model.AuthorInput) (model.Author, error)
	FindByID(context.Context, uint) (model.Author, error)
	List(context.Context) ([]model.Author, error)
}
type authorRepository struct{ db *gorm.DB }

func NewAuthorRepository(db *gorm.DB) AuthorRepository { return &authorRepository{db} }
func (r *authorRepository) Create(ctx context.Context, in model.AuthorInput) (model.Author, error) {
	a := model.Author{FullName: in.FullName, Bio: in.Bio}
	if strings.TrimSpace(in.FullName) == "" {
		return a, ErrValidation
	}
	err := r.db.WithContext(ctx).Create(&a).Error
	return a, translate(err)
}
func (r *authorRepository) FindByID(ctx context.Context, id uint) (model.Author, error) {
	var a model.Author
	err := r.db.WithContext(ctx).First(&a, id).Error
	return a, translate(err)
}
func (r *authorRepository) List(ctx context.Context) ([]model.Author, error) {
	a := make([]model.Author, 0)
	err := r.db.WithContext(ctx).Order("id ASC").Find(&a).Error
	return a, translate(err)
}
