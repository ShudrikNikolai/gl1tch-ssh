package srv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

type fakeRepo struct {
	createFn            func(context.Context, service.User) error
	findByFingerprintFn func(context.Context, string) (service.User, error)
	findByNicknameFn    func(context.Context, string) (service.User, error)
	getByIDsFn          func(context.Context, []uuid.UUID) (map[uuid.UUID]service.User, error)
}

func (f *fakeRepo) Create(ctx context.Context, user service.User) error {
	if f.createFn == nil {
		return nil
	}

	return f.createFn(ctx, user)
}

func (f *fakeRepo) FindByFingerprint(ctx context.Context, fingerprint string) (service.User, error) {
	if f.findByFingerprintFn == nil {
		return service.User{}, repository.ErrNotFound
	}

	return f.findByFingerprintFn(ctx, fingerprint)
}

func (f *fakeRepo) FindByNickname(ctx context.Context, nickname string) (service.User, error) {
	if f.findByNicknameFn == nil {
		return service.User{}, repository.ErrNotFound
	}

	return f.findByNicknameFn(ctx, nickname)
}

func (f *fakeRepo) GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]service.User, error) {
	if f.getByIDsFn == nil {
		return map[uuid.UUID]service.User{}, nil
	}

	return f.getByIDsFn(ctx, ids)
}

func newTestUsers(t *testing.T, repo repository.Repository) *Users {
	t.Helper()

	return New(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), 18)
}

func testFingerprint() string {
	return strings.Repeat("ab", 32)
}

func TestRegister(t *testing.T) {
	t.Run("creates user", func(t *testing.T) {
		var saved service.User

		repo := &fakeRepo{
			createFn: func(_ context.Context, user service.User) error {
				saved = user

				return nil
			},
		}

		user, err := newTestUsers(t, repo).Register(context.Background(), testFingerprint(), "  alice  ")
		if err != nil {
			t.Fatalf("register: %v", err)
		}

		if user.Nickname != "alice" {
			t.Fatalf("nickname = %q, want alice", user.Nickname)
		}
		if saved.Nickname != "alice" {
			t.Fatalf("saved nickname = %q, want alice", saved.Nickname)
		}
		if user.ID == uuid.Nil {
			t.Fatal("user id is empty")
		}
		if user.ID.Version() != 7 {
			t.Fatalf("uuid version = %d, want 7", user.ID.Version())
		}
		if user.CreatedAt.IsZero() {
			t.Fatal("created_at is zero")
		}
		if user.Fingerprint != testFingerprint() {
			t.Fatalf("fingerprint = %q", user.Fingerprint)
		}
	})

	t.Run("rejects invalid nickname", func(t *testing.T) {
		tests := []struct {
			name     string
			nickname string
		}{
			{name: "short", nickname: "ab"},
			{name: "empty", nickname: ""},
			{name: "spaces", nickname: "   "},
			{name: "long", nickname: strings.Repeat("a", 19)},
			{name: "space inside", nickname: "bad nick"},
			{name: "cyrillic", nickname: "ник"},
			{name: "symbol", nickname: "alice!"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := &fakeRepo{
					createFn: func(context.Context, service.User) error {
						t.Fatal("repository must not be called")

						return nil
					},
				}

				_, err := newTestUsers(t, repo).Register(context.Background(), testFingerprint(), tt.nickname)
				if !errors.Is(err, service.ErrInvalidNickname) {
					t.Fatalf("error = %v, want ErrInvalidNickname", err)
				}
			})
		}
	})

	t.Run("rejects invalid fingerprint", func(t *testing.T) {
		tests := []string{
			"",
			strings.Repeat("a", 63),
			strings.Repeat("z", 64),
			"not-a-fingerprint",
		}

		for _, fingerprint := range tests {
			_, err := newTestUsers(t, &fakeRepo{}).Register(context.Background(), fingerprint, "alice")
			if !errors.Is(err, service.ErrInvalidFingerprint) {
				t.Fatalf("fingerprint %q: error = %v, want ErrInvalidFingerprint", fingerprint, err)
			}
		}
	})

	t.Run("maps nickname conflict", func(t *testing.T) {
		repo := &fakeRepo{
			createFn: func(context.Context, service.User) error {
				return repository.ErrNicknameTaken
			},
		}

		_, err := newTestUsers(t, repo).Register(context.Background(), testFingerprint(), "alice")
		if !errors.Is(err, service.ErrNickTaken) {
			t.Fatalf("error = %v, want ErrNickTaken", err)
		}
	})

	t.Run("maps fingerprint conflict", func(t *testing.T) {
		repo := &fakeRepo{
			createFn: func(context.Context, service.User) error {
				return repository.ErrFingerprintTaken
			},
		}

		_, err := newTestUsers(t, repo).Register(context.Background(), testFingerprint(), "alice")
		if !errors.Is(err, service.ErrAlreadyRegistered) {
			t.Fatalf("error = %v, want ErrAlreadyRegistered", err)
		}
	})

	t.Run("wraps unexpected repository error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			createFn: func(context.Context, service.User) error {
				return dbErr
			},
		}

		_, err := newTestUsers(t, repo).Register(context.Background(), testFingerprint(), "alice")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestByFingerprint(t *testing.T) {
	expected := service.User{ID: uuid.New(), Nickname: "alice", Fingerprint: testFingerprint()}

	t.Run("returns user", func(t *testing.T) {
		repo := &fakeRepo{
			findByFingerprintFn: func(context.Context, string) (service.User, error) {
				return expected, nil
			},
		}

		user, err := newTestUsers(t, repo).ByFingerprint(context.Background(), testFingerprint())
		if err != nil {
			t.Fatalf("by fingerprint: %v", err)
		}
		if user != expected {
			t.Fatalf("user = %+v, want %+v", user, expected)
		}
	})

	t.Run("maps not found", func(t *testing.T) {
		repo := &fakeRepo{
			findByFingerprintFn: func(context.Context, string) (service.User, error) {
				return service.User{}, repository.ErrNotFound
			},
		}

		_, err := newTestUsers(t, repo).ByFingerprint(context.Background(), testFingerprint())
		if !errors.Is(err, service.ErrUserNotFound) {
			t.Fatalf("error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("rejects invalid fingerprint", func(t *testing.T) {
		repo := &fakeRepo{
			findByFingerprintFn: func(context.Context, string) (service.User, error) {
				t.Fatal("repository must not be called")

				return service.User{}, nil
			},
		}

		_, err := newTestUsers(t, repo).ByFingerprint(context.Background(), "bad")
		if !errors.Is(err, service.ErrInvalidFingerprint) {
			t.Fatalf("error = %v, want ErrInvalidFingerprint", err)
		}
	})
}

func TestByNickname(t *testing.T) {
	expected := service.User{ID: uuid.New(), Nickname: "alice", Fingerprint: testFingerprint()}

	t.Run("returns user", func(t *testing.T) {
		repo := &fakeRepo{
			findByNicknameFn: func(_ context.Context, nickname string) (service.User, error) {
				if nickname != "alice" {
					t.Fatalf("nickname = %q, want alice", nickname)
				}

				return expected, nil
			},
		}

		user, err := newTestUsers(t, repo).ByNickname(context.Background(), " alice ")
		if err != nil {
			t.Fatalf("by nickname: %v", err)
		}
		if user != expected {
			t.Fatalf("user = %+v, want %+v", user, expected)
		}
	})

	t.Run("maps not found", func(t *testing.T) {
		repo := &fakeRepo{
			findByNicknameFn: func(context.Context, string) (service.User, error) {
				return service.User{}, repository.ErrNotFound
			},
		}

		_, err := newTestUsers(t, repo).ByNickname(context.Background(), "alice")
		if !errors.Is(err, service.ErrUserNotFound) {
			t.Fatalf("error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("rejects invalid nickname", func(t *testing.T) {
		repo := &fakeRepo{
			findByNicknameFn: func(context.Context, string) (service.User, error) {
				t.Fatal("repository must not be called")

				return service.User{}, nil
			},
		}

		_, err := newTestUsers(t, repo).ByNickname(context.Background(), "a")
		if !errors.Is(err, service.ErrInvalidNickname) {
			t.Fatalf("error = %v, want ErrInvalidNickname", err)
		}
	})
}

func TestByIDs(t *testing.T) {
	t.Run("empty list skips repository", func(t *testing.T) {
		repo := &fakeRepo{
			getByIDsFn: func(context.Context, []uuid.UUID) (map[uuid.UUID]service.User, error) {
				t.Fatal("repository must not be called")

				return nil, nil
			},
		}

		users, err := newTestUsers(t, repo).ByIDs(context.Background(), nil)
		if err != nil {
			t.Fatalf("by ids: %v", err)
		}
		if len(users) != 0 {
			t.Fatalf("users = %v, want empty map", users)
		}
	})

	t.Run("returns users map", func(t *testing.T) {
		first := service.User{ID: uuid.New(), Nickname: "alice"}
		second := service.User{ID: uuid.New(), Nickname: "bob"}
		repo := &fakeRepo{
			getByIDsFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]service.User, error) {
				if len(ids) != 2 {
					t.Fatalf("ids = %v, want 2", ids)
				}

				return map[uuid.UUID]service.User{
					first.ID:  first,
					second.ID: second,
				}, nil
			},
		}

		users, err := newTestUsers(t, repo).ByIDs(context.Background(), []uuid.UUID{first.ID, second.ID})
		if err != nil {
			t.Fatalf("by ids: %v", err)
		}
		if len(users) != 2 || users[first.ID].Nickname != "alice" || users[second.ID].Nickname != "bob" {
			t.Fatalf("users = %+v", users)
		}
	})

	t.Run("wraps repository error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			getByIDsFn: func(context.Context, []uuid.UUID) (map[uuid.UUID]service.User, error) {
				return nil, dbErr
			},
		}

		_, err := newTestUsers(t, repo).ByIDs(context.Background(), []uuid.UUID{uuid.New()})
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}
