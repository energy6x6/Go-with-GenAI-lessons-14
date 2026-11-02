package model

import "time"

type Loan struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	CopyID     uint       `json:"copy_id"`
	UserID     uint       `json:"user_id"`
	BorrowedAt time.Time  `json:"borrowed_at"`
	DueAt      time.Time  `json:"due_at"`
	ReturnedAt *time.Time `json:"returned_at"`
}

type BorrowRequest struct {
	CopyID uint `json:"copy_id" binding:"required,gt=0"`
	UserID uint `json:"user_id" binding:"required,gt=0"`
}
