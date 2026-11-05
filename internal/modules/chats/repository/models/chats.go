package models

import (
	"time"

	"github.com/google/uuid"
)

// Chat — модель строки таблицы chats.
type Chat struct {
	ID        uuid.UUID
	HostID    uuid.UUID
	Token     uuid.UUID
	CreatedAt time.Time
	DeletedAt *time.Time
}
