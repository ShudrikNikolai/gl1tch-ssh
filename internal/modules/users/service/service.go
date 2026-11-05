package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNickTaken          = errors.New("nickname already taken")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidNickname    = errors.New("invalid nickname")
	ErrInvalidFingerprint = errors.New("invalid fingerprint")
	ErrAlreadyRegistered  = errors.New("fingerprint already registered")
)

type Service interface {
	Register(ctx context.Context, fingerprint, nickname string) (User, error)
	ByFingerprint(ctx context.Context, fingerprint string) (User, error)
	ByNickname(ctx context.Context, nickname string) (User, error)
	ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]User, error)
}
