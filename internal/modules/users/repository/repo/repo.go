package repo

import (
	"context"
	"errors"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository/converter"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

type Repo struct {
	client *client
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{client: newClient(pool)}
}

func (r *Repo) Create(ctx context.Context, user service.User) error {
	return mapError(r.client.create(ctx, converter.ToModel(user)))
}

func (r *Repo) FindByFingerprint(ctx context.Context, fingerprint string) (service.User, error) {
	user, err := r.client.findByFingerprint(ctx, fingerprint)
	if err != nil {
		return service.User{}, mapError(err)
	}

	return converter.ToDomain(user), nil
}

func (r *Repo) FindByNickname(ctx context.Context, nickname string) (service.User, error) {
	user, err := r.client.findByNickname(ctx, nickname)
	if err != nil {
		return service.User{}, mapError(err)
	}

	return converter.ToDomain(user), nil
}

func (r *Repo) GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]service.User, error) {
	rows, err := r.client.getByIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}

	users := make(map[uuid.UUID]service.User, len(rows))
	for _, row := range rows {
		user := converter.ToDomain(row)
		users[user.ID] = user
	}

	return users, nil
}

func mapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case "users_nickname_key":
			return repository.ErrNicknameTaken
		case "users_fingerprint_key":
			return repository.ErrFingerprintTaken
		}
	}

	return err
}
