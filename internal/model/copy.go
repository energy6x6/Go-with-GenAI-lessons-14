package model

import "time"

type Copy struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	BookID      uint      `json:"book_id" gorm:"not null;index"`
	InventoryNo string    `json:"inventory_no" gorm:"uniqueIndex;not null"`
	Status      string    `json:"status" gorm:"not null;default:available"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type CopyInput struct {
	InventoryNo string `json:"inventory_no" binding:"required"`
	Status      string `json:"status"`
}

func ValidCopyStatus(s string) bool { return s == "available" || s == "borrowed" || s == "lost" }

type Availability struct {
	BookID          uint  `json:"book_id"`
	TotalCopies     int64 `json:"total_copies"`
	AvailableCopies int64 `json:"available_copies"`
}
