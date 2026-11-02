package model

import (
	"strings"
	"time"
)

type Book struct {
	AvailableCopies int64     `json:"-" gorm:"->;-:migration"`
	TotalCopies     int64     `json:"-" gorm:"->;-:migration"`
	ID              uint      `json:"id" gorm:"primaryKey"`
	Title           string    `json:"title" gorm:"not null"`
	ISBN            string    `json:"isbn" gorm:"uniqueIndex;not null"`
	CategoryID      uint      `json:"category_id" gorm:"not null;index"`
	Category        Category  `json:"category" gorm:"foreignKey:CategoryID"`
	Authors         []Author  `json:"authors" gorm:"many2many:book_authors;"`
	PublishedYear   *int      `json:"published_year"`
	Description     *string   `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type BookInput struct {
	Title         string  `json:"title" binding:"required"`
	ISBN          string  `json:"isbn" binding:"required"`
	CategoryID    uint    `json:"category_id" binding:"required"`
	AuthorIDs     []uint  `json:"author_ids" binding:"required,min=1"`
	PublishedYear *int    `json:"published_year"`
	Description   *string `json:"description"`
}

func (b BookInput) Valid() bool {
	if strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.ISBN) == "" || b.CategoryID == 0 || len(b.AuthorIDs) == 0 {
		return false
	}
	seen := make(map[uint]bool, len(b.AuthorIDs))
	for _, id := range b.AuthorIDs {
		if id == 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
