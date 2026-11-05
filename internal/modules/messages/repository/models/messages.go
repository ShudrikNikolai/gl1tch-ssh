package models

import (
	"time"

	"github.com/google/uuid"
)

// Message — модель строки таблицы messages.
type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	AuthorID  uuid.UUID
	Text      string
	CreatedAt time.Time
}
