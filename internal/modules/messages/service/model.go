package service

import (
	"time"

	"github.com/google/uuid"
)

// Message — доменная модель сообщения.
type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	AuthorID  uuid.UUID
	Text      string
	CreatedAt time.Time
}

// Cursor — позиция для пагинации истории.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}
