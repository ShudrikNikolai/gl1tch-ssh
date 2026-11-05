package repo

import (
	"context"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertQuery = `INSERT INTO messages (id, chat_id, author_id, text, created_at)
	               VALUES ($1, $2, $3, $4, $5)`

	listLatestQuery = `SELECT id, chat_id, author_id, text, created_at
	                   FROM messages
	                   WHERE chat_id = $1
	                   ORDER BY created_at DESC, id DESC
	                   LIMIT $2`

	listBeforeQuery = `SELECT id, chat_id, author_id, text, created_at
	                   FROM messages
	                   WHERE chat_id = $1 AND (created_at, id) < ($2, $3)
	                   ORDER BY created_at DESC, id DESC
	                   LIMIT $4`
)

type client struct {
	pool *pgxpool.Pool
}

func newClient(pool *pgxpool.Pool) *client {
	return &client{pool: pool}
}

func (c *client) insert(ctx context.Context, msg models.Message) error {
	_, err := c.pool.Exec(
		ctx,
		insertQuery,
		msg.ID,
		msg.ChatID,
		msg.AuthorID,
		msg.Text,
		msg.CreatedAt,
	)

	return err
}

func (c *client) listLatest(ctx context.Context, chatID uuid.UUID, limit int) ([]models.Message, error) {
	rows, err := c.pool.Query(ctx, listLatestQuery, chatID, limit)
	if err != nil {
		return nil, err
	}

	return collectMessages(rows)
}

func (c *client) listBefore(
	ctx context.Context,
	chatID uuid.UUID,
	beforeAt time.Time,
	beforeID uuid.UUID,
	limit int,
) ([]models.Message, error) {
	rows, err := c.pool.Query(ctx, listBeforeQuery, chatID, beforeAt, beforeID, limit)
	if err != nil {
		return nil, err
	}

	return collectMessages(rows)
}

func collectMessages(rows pgx.Rows) ([]models.Message, error) {
	defer rows.Close()

	messages := make([]models.Message, 0)
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.AuthorID, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}
