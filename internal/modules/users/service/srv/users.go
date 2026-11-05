package srv

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

const (
	minNickLen      = 3
	defaultNickLen  = 18
	fingerprintSize = 32
)

type Users struct {
	repo       repository.Repository
	log        *slog.Logger
	maxNickLen int
}

func New(repo repository.Repository, log *slog.Logger, maxNickLen int) *Users {
	if maxNickLen < minNickLen {
		maxNickLen = defaultNickLen
	}

	return &Users{
		repo:       repo,
		log:        log,
		maxNickLen: maxNickLen,
	}
}

func (u *Users) Register(ctx context.Context, fingerprint, nickname string) (service.User, error) {
	nickname = strings.TrimSpace(nickname)
	if err := validateNickname(nickname, u.maxNickLen); err != nil {
		return service.User{}, err
	}
	if !validFingerprint(fingerprint) {
		return service.User{}, service.ErrInvalidFingerprint
	}

	id, err := uuid.NewV7()
	if err != nil {
		return service.User{}, fmt.Errorf("generate user id: %w", err)
	}

	user := service.User{
		ID:          id,
		Nickname:    nickname,
		Fingerprint: fingerprint,
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}

	if err = u.repo.Create(ctx, user); err != nil {
		switch {
		case errors.Is(err, repository.ErrNicknameTaken):
			return service.User{}, service.ErrNickTaken
		case errors.Is(err, repository.ErrFingerprintTaken):
			return service.User{}, service.ErrAlreadyRegistered
		default:
			return service.User{}, fmt.Errorf("create user: %w", err)
		}
	}

	u.log.Debug("user registered", "user_id", id, "nickname", nickname)

	return user, nil
}

func (u *Users) ByFingerprint(ctx context.Context, fingerprint string) (service.User, error) {
	if !validFingerprint(fingerprint) {
		return service.User{}, service.ErrInvalidFingerprint
	}

	user, err := u.repo.FindByFingerprint(ctx, fingerprint)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return service.User{}, service.ErrUserNotFound
		}

		return service.User{}, fmt.Errorf("find user by fingerprint: %w", err)
	}

	return user, nil
}

func (u *Users) ByNickname(ctx context.Context, nickname string) (service.User, error) {
	nickname = strings.TrimSpace(nickname)
	if err := validateNickname(nickname, u.maxNickLen); err != nil {
		return service.User{}, err
	}

	user, err := u.repo.FindByNickname(ctx, nickname)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return service.User{}, service.ErrUserNotFound
		}

		return service.User{}, fmt.Errorf("find user by nickname: %w", err)
	}

	return user, nil
}

func (u *Users) ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]service.User, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]service.User{}, nil
	}

	users, err := u.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}

	return users, nil
}

func validateNickname(nickname string, maxLen int) error {
	length := utf8.RuneCountInString(nickname)
	if length < minNickLen || length > maxLen {
		return service.ErrInvalidNickname
	}

	for _, r := range nickname {
		isLetter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isLetter && !isDigit && r != '_' && r != '-' {
			return service.ErrInvalidNickname
		}
	}

	return nil
}

func validFingerprint(fingerprint string) bool {
	if len(fingerprint) != fingerprintSize*2 {
		return false
	}

	_, err := hex.DecodeString(fingerprint)

	return err == nil
}
