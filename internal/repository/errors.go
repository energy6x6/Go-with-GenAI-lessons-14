package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("resource not found")

var (
	ErrConflict         = errors.New("resource conflicts with existing data")
	ErrInvalidReference = errors.New("referenced resource does not exist")
	ErrValidation       = errors.New("invalid resource")
)

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		switch pgerr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrInvalidReference
		case "23514", "23502", "22001", "22003":
			return ErrValidation
		}
	}
	return err
}
