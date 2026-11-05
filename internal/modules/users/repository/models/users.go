package models

import (
	"time"

	"github.com/google/uuid"
)

// User — модель строки таблицы users.
type User struct {
	ID          uuid.UUID
	Nickname    string
	Fingerprint string
	CreatedAt   time.Time
}
