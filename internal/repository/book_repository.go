package repository

import (
	"context"
	"libratrack/internal/model"
)

type BookFilter struct {
	Search              string
	IncludeAvailability bool
	CategoryID          uint
	Page, Limit         uint64
}
type BookRepository interface {
	Create(context.Context, model.BookInput) (model.Book, error)
	FindByID(context.Context, uint) (model.Book, error)
	List(context.Context, BookFilter) ([]model.Book, error)
	Update(context.Context, uint, model.BookInput) (model.Book, error)
	Delete(context.Context, uint) error
}
