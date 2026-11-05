package service

import (
	"time"

	"github.com/google/uuid"
)

// Chat — доменная модель комнаты.
type Chat struct {
	ID        uuid.UUID
	HostID    uuid.UUID
	Token     uuid.UUID
	CreatedAt time.Time
	DeletedAt *time.Time
}
