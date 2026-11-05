package service

import (
	"context"

	"github.com/google/uuid"
)

type Broadcaster interface {
	Publish(ctx context.Context, chatID uuid.UUID, msg Message) error
	Subscribe(ctx context.Context, chatID uuid.UUID) (<-chan Message, func(), error)
}
