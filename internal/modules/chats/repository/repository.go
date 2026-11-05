package repository

import (
	"context"
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("chats: not found")
	ErrNotHost       = errors.New("chats: user is not host")
	ErrFull          = errors.New("chats: chat is full")
	ErrBanned        = errors.New("chats: user is banned")
	ErrNotMember     = errors.New("chats: user is not a member")
	ErrCannotBanHost = errors.New("chats: cannot ban host")
)

type Repository interface {
	Create(ctx context.Context, chat service.Chat) error
	GetByID(ctx context.Context, chatID uuid.UUID) (service.Chat, error)
	Join(ctx context.Context, token, userID uuid.UUID, maxMembers int) (service.Chat, error)
	Delete(ctx context.Context, chatID, hostID uuid.UUID) error
	Ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]service.Chat, error)
	IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	Members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error)
}
