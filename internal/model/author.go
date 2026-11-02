package model

import "time"

type Author struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	FullName  string    `json:"full_name" gorm:"not null"`
	Bio       *string   `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuthorInput struct {
	FullName string  `json:"full_name" binding:"required"`
	Bio      *string `json:"bio"`
}
