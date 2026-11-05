package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrChatNotFound  = errors.New("chat not found")
	ErrChatFull      = errors.New("chat is full")
	ErrBanned        = errors.New("user is banned in chat")
	ErrNotHost       = errors.New("user is not chat host")
	ErrUserNotMember = errors.New("user is not a chat member")
	ErrCannotBanHost = errors.New("cannot ban chat host")
	ErrInvalidToken  = errors.New("invalid chat token")
)

type Service interface {
	Create(ctx context.Context, hostID uuid.UUID) (Chat, error)
	Join(ctx context.Context, token, userID uuid.UUID) (Chat, error)
	ByID(ctx context.Context, chatID uuid.UUID) (Chat, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Chat, error)
	Delete(ctx context.Context, chatID, hostID uuid.UUID) error
	Ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error
	IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	Members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error)
}
