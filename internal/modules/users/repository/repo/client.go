package repo

import (
	"context"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type client struct {
	pool *pgxpool.Pool
}

func newClient(pool *pgxpool.Pool) *client {
	return &client{pool: pool}
}

func (c *client) create(ctx context.Context, user models.User) error {
	_, err := c.pool.Exec(
		ctx,
		`INSERT INTO users (id, nickname, fingerprint, created_at) VALUES ($1, $2, $3, $4)`,
		user.ID,
		user.Nickname,
		user.Fingerprint,
		user.CreatedAt,
	)

	return err
}

func (c *client) findByFingerprint(ctx context.Context, fingerprint string) (models.User, error) {
	var user models.User

	err := c.pool.QueryRow(
		ctx,
		`SELECT id, nickname, fingerprint, created_at FROM users WHERE fingerprint = $1`,
		fingerprint,
	).Scan(&user.ID, &user.Nickname, &user.Fingerprint, &user.CreatedAt)

	return user, err
}

func (c *client) findByNickname(ctx context.Context, nickname string) (models.User, error) {
	var user models.User

	err := c.pool.QueryRow(
		ctx,
		`SELECT id, nickname, fingerprint, created_at FROM users WHERE nickname = $1`,
		nickname,
	).Scan(&user.ID, &user.Nickname, &user.Fingerprint, &user.CreatedAt)

	return user, err
}

func (c *client) getByIDs(ctx context.Context, ids []uuid.UUID) ([]models.User, error) {
	rows, err := c.pool.Query(
		ctx,
		`SELECT id, nickname, fingerprint, created_at FROM users WHERE id = ANY($1)`,
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.User, 0, len(ids))
	for rows.Next() {
		var user models.User
		if err = rows.Scan(&user.ID, &user.Nickname, &user.Fingerprint, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}
