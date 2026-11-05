package repo

import (
	"context"
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository/converter"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	client *client
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{client: newClient(pool)}
}

func (r *Repo) Create(ctx context.Context, chat service.Chat) error {
	return r.client.create(ctx, converter.ToModel(chat))
}

func (r *Repo) GetByID(ctx context.Context, chatID uuid.UUID) (service.Chat, error) {
	chat, err := r.client.getByID(ctx, chatID)
	if err != nil {
		return service.Chat{}, mapError(err)
	}

	return converter.ToDomain(chat), nil
}

func (r *Repo) Join(ctx context.Context, token, userID uuid.UUID, maxMembers int) (service.Chat, error) {
	chat, err := r.client.join(ctx, token, userID, maxMembers)
	if err != nil {
		return service.Chat{}, err
	}

	return converter.ToDomain(chat), nil
}

func (r *Repo) Delete(ctx context.Context, chatID, hostID uuid.UUID) error {
	return r.client.delete(ctx, chatID, hostID)
}

func (r *Repo) Ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error {
	return r.client.ban(ctx, chatID, hostID, targetID)
}

func (r *Repo) ListByUser(ctx context.Context, userID uuid.UUID) ([]service.Chat, error) {
	chats, err := r.client.listByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return converter.ToDomains(chats), nil
}

func (r *Repo) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	return r.client.isMember(ctx, chatID, userID)
}

func (r *Repo) Members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error) {
	return r.client.members(ctx, chatID)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}

	return err
}
