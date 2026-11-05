package repository

import (
	"context"
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

var (
	ErrNotFound         = errors.New("users: not found")
	ErrNicknameTaken    = errors.New("users: nickname taken")
	ErrFingerprintTaken = errors.New("users: fingerprint taken")
)

type Repository interface {
	Create(ctx context.Context, user service.User) error
	FindByFingerprint(ctx context.Context, fingerprint string) (service.User, error)
	FindByNickname(ctx context.Context, nickname string) (service.User, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]service.User, error)
}
