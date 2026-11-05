package srv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
)

const defaultMaxMembers = 4

type Chats struct {
	repo       repository.Repository
	log        *slog.Logger
	maxMembers int
}

func New(repo repository.Repository, log *slog.Logger, maxMembers int) *Chats {
	if maxMembers <= 0 {
		maxMembers = defaultMaxMembers
	}

	return &Chats{
		repo:       repo,
		log:        log,
		maxMembers: maxMembers,
	}
}

func (c *Chats) Create(ctx context.Context, hostID uuid.UUID) (service.Chat, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return service.Chat{}, fmt.Errorf("generate chat id: %w", err)
	}

	token, err := uuid.NewV7()
	if err != nil {
		return service.Chat{}, fmt.Errorf("generate chat token: %w", err)
	}

	chat := service.Chat{
		ID:        id,
		HostID:    hostID,
		Token:     token,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	if err = c.repo.Create(ctx, chat); err != nil {
		return service.Chat{}, fmt.Errorf("create chat: %w", err)
	}

	c.log.Debug("chat created", "chat_id", id, "host_id", hostID)

	return chat, nil
}

func (c *Chats) Join(ctx context.Context, token, userID uuid.UUID) (service.Chat, error) {
	if token == uuid.Nil {
		return service.Chat{}, service.ErrInvalidToken
	}

	chat, err := c.repo.Join(ctx, token, userID, c.maxMembers)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return service.Chat{}, service.ErrChatNotFound
		case errors.Is(err, repository.ErrBanned):
			return service.Chat{}, service.ErrBanned
		case errors.Is(err, repository.ErrFull):
			return service.Chat{}, service.ErrChatFull
		default:
			return service.Chat{}, fmt.Errorf("join chat: %w", err)
		}
	}

	return chat, nil
}

func (c *Chats) ByID(ctx context.Context, chatID uuid.UUID) (service.Chat, error) {
	chat, err := c.repo.GetByID(ctx, chatID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return service.Chat{}, service.ErrChatNotFound
		}

		return service.Chat{}, fmt.Errorf("get chat: %w", err)
	}

	return chat, nil
}

func (c *Chats) ListByUser(ctx context.Context, userID uuid.UUID) ([]service.Chat, error) {
	chats, err := c.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}

	return chats, nil
}

func (c *Chats) Delete(ctx context.Context, chatID, hostID uuid.UUID) error {
	if err := c.repo.Delete(ctx, chatID, hostID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return service.ErrChatNotFound
		case errors.Is(err, repository.ErrNotHost):
			return service.ErrNotHost
		default:
			return fmt.Errorf("delete chat: %w", err)
		}
	}

	return nil
}

func (c *Chats) Ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error {
	if err := c.repo.Ban(ctx, chatID, hostID, targetID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return service.ErrChatNotFound
		case errors.Is(err, repository.ErrNotHost):
			return service.ErrNotHost
		case errors.Is(err, repository.ErrCannotBanHost):
			return service.ErrCannotBanHost
		case errors.Is(err, repository.ErrNotMember):
			return service.ErrUserNotMember
		default:
			return fmt.Errorf("ban user: %w", err)
		}
	}

	return nil
}

func (c *Chats) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	member, err := c.repo.IsMember(ctx, chatID, userID)
	if err != nil {
		return false, fmt.Errorf("check chat member: %w", err)
	}

	return member, nil
}

func (c *Chats) Members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error) {
	members, err := c.repo.Members(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("list chat members: %w", err)
	}

	return members, nil
}
