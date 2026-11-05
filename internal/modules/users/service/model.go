package service

import (
	"time"

	"github.com/google/uuid"
)

// User — доменная модель пользователя.
type User struct {
	ID          uuid.UUID
	Nickname    string
	Fingerprint string
	CreatedAt   time.Time
}
