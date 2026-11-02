package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"libratrack/internal/model"
)

type UserRepository interface {
	Create(context.Context, *model.User) error
	FindByEmail(context.Context, string) (model.User, error)
	FindByID(context.Context, uint) (model.User, error)
	List(context.Context, int, int) ([]model.User, error)
	SetRole(context.Context, uint, string) (model.User, error)
	UpdateProfile(context.Context, uint, model.UpdateProfileRequest) (model.User, error)
}

type userRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepository{db} }
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	return translate(r.db.WithContext(ctx).Create(user).Error)
}
func (r *userRepository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	return user, translate(err)
}
func (r *userRepository) FindByID(ctx context.Context, id uint) (model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	return user, translate(err)
}
func (r *userRepository) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	users := make([]model.User, 0)
	err := r.db.WithContext(ctx).Select("id", "email", "name", "role", "created_at", "updated_at").Order("id ASC").Limit(limit).Offset(offset).Find(&users).Error
	return users, translate(err)
}
func (r *userRepository) SetRole(ctx context.Context, id uint, role string) (model.User, error) {
	if !model.ValidRole(role) {
		return model.User{}, ErrValidation
	}
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{"role": role, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return model.User{}, translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return model.User{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}
func (r *userRepository) UpdateProfile(ctx context.Context, id uint, in model.UpdateProfileRequest) (model.User, error) {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{"name": in.Name, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return model.User{}, translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return model.User{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}
