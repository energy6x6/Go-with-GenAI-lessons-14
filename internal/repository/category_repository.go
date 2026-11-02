package repository

import (
	"context"
	"gorm.io/gorm"
	"libratrack/internal/model"
	"strings"
)

type CategoryRepository interface {
	Create(context.Context, model.CategoryInput) (model.Category, error)
	FindByID(context.Context, uint) (model.Category, error)
	List(context.Context) ([]model.Category, error)
}
type categoryRepository struct{ db *gorm.DB }

func NewCategoryRepository(db *gorm.DB) CategoryRepository { return &categoryRepository{db} }
func (r *categoryRepository) Create(ctx context.Context, in model.CategoryInput) (model.Category, error) {
	c := model.Category{Name: in.Name}
	if strings.TrimSpace(in.Name) == "" {
		return c, ErrValidation
	}
	err := r.db.WithContext(ctx).Create(&c).Error
	return c, translate(err)
}
func (r *categoryRepository) FindByID(ctx context.Context, id uint) (model.Category, error) {
	var c model.Category
	err := r.db.WithContext(ctx).First(&c, id).Error
	return c, translate(err)
}
func (r *categoryRepository) List(ctx context.Context) ([]model.Category, error) {
	c := make([]model.Category, 0)
	err := r.db.WithContext(ctx).Order("id ASC").Find(&c).Error
	return c, translate(err)
}
