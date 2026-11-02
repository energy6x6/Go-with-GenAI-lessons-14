package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"libratrack/internal/model"
)

type LoanRepository interface {
	Borrow(context.Context, model.BorrowRequest, time.Time) (model.Loan, error)
	Return(context.Context, uint, time.Time) (model.Loan, error)
	ForUser(context.Context, uint, int, int) ([]model.Loan, error)
}
type loanRepository struct{ db *gorm.DB }

func NewLoanRepository(db *gorm.DB) LoanRepository { return &loanRepository{db} }
func (r *loanRepository) Borrow(ctx context.Context, in model.BorrowRequest, due time.Time) (model.Loan, error) {
	var loan model.Loan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).First(&user, in.UserID).Error; err != nil {
			return err
		}
		var copy model.Copy
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&copy, in.CopyID).Error; err != nil {
			return err
		}
		if copy.Status != "available" {
			return ErrConflict
		}
		loan = model.Loan{CopyID: in.CopyID, UserID: in.UserID, BorrowedAt: time.Now().UTC(), DueAt: due}
		if err := tx.Create(&loan).Error; err != nil {
			return err
		}
		return tx.Model(&copy).Updates(map[string]any{"status": "borrowed", "updated_at": time.Now().UTC()}).Error
	})
	return loan, translate(err)
}
func (r *loanRepository) Return(ctx context.Context, id uint, now time.Time) (model.Loan, error) {
	var loan model.Loan
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&loan, id).Error; err != nil {
			return err
		}
		if loan.ReturnedAt != nil {
			return ErrConflict
		}
		if err := tx.Model(&loan).Update("returned_at", now).Error; err != nil {
			return err
		}
		loan.ReturnedAt = &now
		return tx.Model(&model.Copy{}).Where("id = ?", loan.CopyID).Updates(map[string]any{"status": "available", "updated_at": now}).Error
	})
	return loan, translate(err)
}
func (r *loanRepository) ForUser(ctx context.Context, userID uint, limit, offset int) ([]model.Loan, error) {
	loans := make([]model.Loan, 0)
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id ASC").Limit(limit).Offset(offset).Find(&loans).Error
	return loans, translate(err)
}
