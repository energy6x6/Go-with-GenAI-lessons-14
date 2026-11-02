package auth

import (
	"context"
	"errors"
	"libratrack/internal/model"
	"libratrack/internal/repository"
)

var ErrCredentials = errors.New("invalid credentials")

type Credentials struct {
	users     repository.UserRepository
	dummyHash string
}

func NewCredentials(users repository.UserRepository) (*Credentials, error) {
	hash, err := HashPassword("dummy-password-for-absent-accounts")
	if err != nil {
		return nil, err
	}
	return &Credentials{users: users, dummyHash: hash}, nil
}

func (v *Credentials) Verify(ctx context.Context, email, password string) (model.User, error) {
	user, err := v.users.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return model.User{}, err
	}
	hash := user.PasswordHash
	if hash == "" {
		hash = v.dummyHash
	}
	passwordErr := CheckPassword(hash, password)
	if err != nil || user.PasswordHash == "" || passwordErr != nil {
		return model.User{}, ErrCredentials
	}
	return user, nil
}
