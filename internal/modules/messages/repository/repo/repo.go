package repo

import (
	"context"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository/converter"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	client *client
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{client: newClient(pool)}
}

func (r *Repo) Insert(ctx context.Context, msg service.Message) error {
	return r.client.insert(ctx, converter.ToModel(msg))
}

func (r *Repo) ListLatest(ctx context.Context, chatID uuid.UUID, limit int) ([]service.Message, error) {
	messages, err := r.client.listLatest(ctx, chatID, limit)
	if err != nil {
		return nil, err
	}

	return converter.ToDomains(messages), nil
}

func (r *Repo) ListBefore(
	ctx context.Context,
	chatID uuid.UUID,
	before service.Cursor,
	limit int,
) ([]service.Message, error) {
	messages, err := r.client.listBefore(ctx, chatID, before.CreatedAt, before.ID, limit)
	if err != nil {
		return nil, err
	}

	return converter.ToDomains(messages), nil
}
