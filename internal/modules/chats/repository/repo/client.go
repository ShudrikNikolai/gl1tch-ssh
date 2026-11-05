package repo

import (
	"context"
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type client struct {
	pool *pgxpool.Pool
}

func newClient(pool *pgxpool.Pool) *client {
	return &client{pool: pool}
}

func (c *client) create(ctx context.Context, chat models.Chat) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(
		ctx,
		`INSERT INTO chats (id, host_id, token, created_at) VALUES ($1, $2, $3, $4)`,
		chat.ID,
		chat.HostID,
		chat.Token,
		chat.CreatedAt,
	); err != nil {
		return err
	}

	if _, err = tx.Exec(
		ctx,
		`INSERT INTO chat_members (chat_id, user_id, joined_at) VALUES ($1, $2, $3)`,
		chat.ID,
		chat.HostID,
		chat.CreatedAt,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (c *client) getByID(ctx context.Context, chatID uuid.UUID) (models.Chat, error) {
	var chat models.Chat

	err := c.pool.QueryRow(
		ctx,
		`SELECT id, host_id, token, created_at, deleted_at
		 FROM chats
		 WHERE id = $1 AND deleted_at IS NULL`,
		chatID,
	).Scan(&chat.ID, &chat.HostID, &chat.Token, &chat.CreatedAt, &chat.DeletedAt)

	return chat, err
}

func (c *client) join(ctx context.Context, token, userID uuid.UUID, maxMembers int) (models.Chat, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return models.Chat{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var chat models.Chat

	err = tx.QueryRow(
		ctx,
		`SELECT id, host_id, token, created_at, deleted_at
		 FROM chats
		 WHERE token = $1 AND deleted_at IS NULL
		 FOR UPDATE`,
		token,
	).Scan(&chat.ID, &chat.HostID, &chat.Token, &chat.CreatedAt, &chat.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Chat{}, repository.ErrNotFound
	}
	if err != nil {
		return models.Chat{}, err
	}

	var banned bool
	if err = tx.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM chat_bans WHERE chat_id = $1 AND user_id = $2)`,
		chat.ID,
		userID,
	).Scan(&banned); err != nil {
		return models.Chat{}, err
	}
	if banned {
		return models.Chat{}, repository.ErrBanned
	}

	var member bool
	if err = tx.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`,
		chat.ID,
		userID,
	).Scan(&member); err != nil {
		return models.Chat{}, err
	}
	if member {
		return chat, tx.Commit(ctx)
	}

	var count int
	if err = tx.QueryRow(
		ctx,
		`SELECT count(*) FROM chat_members WHERE chat_id = $1`,
		chat.ID,
	).Scan(&count); err != nil {
		return models.Chat{}, err
	}
	if count >= maxMembers {
		return models.Chat{}, repository.ErrFull
	}

	if _, err = tx.Exec(
		ctx,
		`INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2)`,
		chat.ID,
		userID,
	); err != nil {
		return models.Chat{}, err
	}

	return chat, tx.Commit(ctx)
}

func (c *client) delete(ctx context.Context, chatID, hostID uuid.UUID) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var actualHost uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT host_id FROM chats WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		chatID,
	).Scan(&actualHost)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if actualHost != hostID {
		return repository.ErrNotHost
	}

	if _, err = tx.Exec(ctx, `UPDATE chats SET deleted_at = now() WHERE id = $1`, chatID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (c *client) ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var actualHost uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT host_id FROM chats WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		chatID,
	).Scan(&actualHost)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if actualHost != hostID {
		return repository.ErrNotHost
	}
	if targetID == actualHost {
		return repository.ErrCannotBanHost
	}

	var member bool
	if err = tx.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`,
		chatID,
		targetID,
	).Scan(&member); err != nil {
		return err
	}
	if !member {
		return repository.ErrNotMember
	}

	if _, err = tx.Exec(
		ctx,
		`INSERT INTO chat_bans (chat_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		chatID,
		targetID,
	); err != nil {
		return err
	}

	if _, err = tx.Exec(
		ctx,
		`DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`,
		chatID,
		targetID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (c *client) listByUser(ctx context.Context, userID uuid.UUID) ([]models.Chat, error) {
	rows, err := c.pool.Query(
		ctx,
		`SELECT c.id, c.host_id, c.token, c.created_at, c.deleted_at
		 FROM chats c
		 JOIN chat_members m ON m.chat_id = c.id
		 WHERE m.user_id = $1 AND c.deleted_at IS NULL
		 ORDER BY c.created_at DESC, c.id DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]models.Chat, 0)
	for rows.Next() {
		var chat models.Chat
		if err = rows.Scan(&chat.ID, &chat.HostID, &chat.Token, &chat.CreatedAt, &chat.DeletedAt); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}

	return chats, rows.Err()
}

func (c *client) isMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	var member bool

	err := c.pool.QueryRow(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`,
		chatID,
		userID,
	).Scan(&member)

	return member, err
}

func (c *client) members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := c.pool.Query(
		ctx,
		`SELECT user_id FROM chat_members WHERE chat_id = $1 ORDER BY joined_at, user_id`,
		chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err = rows.Scan(&userID); err != nil {
			return nil, err
		}
		members = append(members, userID)
	}

	return members, rows.Err()
}
