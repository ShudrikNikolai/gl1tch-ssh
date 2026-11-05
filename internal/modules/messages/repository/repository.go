package repository

import (
	"context"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/google/uuid"
)

type Repository interface {
	Insert(ctx context.Context, msg service.Message) error
	ListLatest(ctx context.Context, chatID uuid.UUID, limit int) ([]service.Message, error)
	ListBefore(ctx context.Context, chatID uuid.UUID, before service.Cursor, limit int) ([]service.Message, error)
}
