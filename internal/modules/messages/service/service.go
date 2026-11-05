package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

const (
	DefaultHistoryLimit = 20
	MaxHistoryLimit     = 100
)

var (
	ErrEmptyMessage   = errors.New("message is empty")
	ErrMessageTooLong = errors.New("message is too long")
)

type Service interface {
	Send(ctx context.Context, chatID, authorID uuid.UUID, text string) (Message, error)
	History(ctx context.Context, chatID uuid.UUID, before *Cursor, limit int) ([]Message, error)
}
