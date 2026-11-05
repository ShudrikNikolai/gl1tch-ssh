package srv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
)

type fakeRepo struct {
	createFn     func(context.Context, service.Chat) error
	getByIDFn    func(context.Context, uuid.UUID) (service.Chat, error)
	joinFn       func(context.Context, uuid.UUID, uuid.UUID, int) (service.Chat, error)
	deleteFn     func(context.Context, uuid.UUID, uuid.UUID) error
	banFn        func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	listByUserFn func(context.Context, uuid.UUID) ([]service.Chat, error)
	isMemberFn   func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	membersFn    func(context.Context, uuid.UUID) ([]uuid.UUID, error)
}

func (f *fakeRepo) Create(ctx context.Context, chat service.Chat) error {
	if f.createFn == nil {
		return nil
	}

	return f.createFn(ctx, chat)
}

func (f *fakeRepo) GetByID(ctx context.Context, chatID uuid.UUID) (service.Chat, error) {
	if f.getByIDFn == nil {
		return service.Chat{}, repository.ErrNotFound
	}

	return f.getByIDFn(ctx, chatID)
}

func (f *fakeRepo) Join(ctx context.Context, token, userID uuid.UUID, maxMembers int) (service.Chat, error) {
	if f.joinFn == nil {
		return service.Chat{}, repository.ErrNotFound
	}

	return f.joinFn(ctx, token, userID, maxMembers)
}

func (f *fakeRepo) Delete(ctx context.Context, chatID, hostID uuid.UUID) error {
	if f.deleteFn == nil {
		return nil
	}

	return f.deleteFn(ctx, chatID, hostID)
}

func (f *fakeRepo) Ban(ctx context.Context, chatID, hostID, targetID uuid.UUID) error {
	if f.banFn == nil {
		return nil
	}

	return f.banFn(ctx, chatID, hostID, targetID)
}

func (f *fakeRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]service.Chat, error) {
	if f.listByUserFn == nil {
		return nil, nil
	}

	return f.listByUserFn(ctx, userID)
}

func (f *fakeRepo) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	if f.isMemberFn == nil {
		return false, nil
	}

	return f.isMemberFn(ctx, chatID, userID)
}

func (f *fakeRepo) Members(ctx context.Context, chatID uuid.UUID) ([]uuid.UUID, error) {
	if f.membersFn == nil {
		return nil, nil
	}

	return f.membersFn(ctx, chatID)
}

func newTestChats(t *testing.T, repo repository.Repository, maxMembers int) *Chats {
	t.Helper()

	return New(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), maxMembers)
}

func TestCreate(t *testing.T) {
	t.Run("creates chat with host member", func(t *testing.T) {
		var saved service.Chat

		repo := &fakeRepo{
			createFn: func(_ context.Context, chat service.Chat) error {
				saved = chat

				return nil
			},
		}
		hostID := uuid.New()

		chat, err := newTestChats(t, repo, 4).Create(context.Background(), hostID)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		if chat.ID == uuid.Nil || chat.ID.Version() != 7 {
			t.Fatalf("chat id = %v, want uuidv7", chat.ID)
		}
		if chat.Token == uuid.Nil || chat.Token.Version() != 7 {
			t.Fatalf("token = %v, want uuidv7", chat.Token)
		}
		if chat.HostID != hostID {
			t.Fatalf("host = %v, want %v", chat.HostID, hostID)
		}
		if chat.CreatedAt.IsZero() {
			t.Fatal("created_at is zero")
		}
		if saved != chat {
			t.Fatalf("saved chat = %+v, want %+v", saved, chat)
		}
	})

	t.Run("wraps repository error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			createFn: func(context.Context, service.Chat) error {
				return dbErr
			},
		}

		_, err := newTestChats(t, repo, 4).Create(context.Background(), uuid.New())
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestJoin(t *testing.T) {
	token := uuid.New()
	userID := uuid.New()
	expected := service.Chat{ID: uuid.New(), HostID: uuid.New(), Token: token}

	t.Run("joins chat", func(t *testing.T) {
		repo := &fakeRepo{
			joinFn: func(_ context.Context, gotToken, gotUserID uuid.UUID, maxMembers int) (service.Chat, error) {
				if gotToken != token || gotUserID != userID {
					t.Fatalf("got token=%v user=%v", gotToken, gotUserID)
				}
				if maxMembers != 4 {
					t.Fatalf("max members = %d, want 4", maxMembers)
				}

				return expected, nil
			},
		}

		chat, err := newTestChats(t, repo, 0).Join(context.Background(), token, userID)
		if err != nil {
			t.Fatalf("join: %v", err)
		}
		if chat != expected {
			t.Fatalf("chat = %+v, want %+v", chat, expected)
		}
	})

	t.Run("passes configured limit", func(t *testing.T) {
		repo := &fakeRepo{
			joinFn: func(_ context.Context, _, _ uuid.UUID, maxMembers int) (service.Chat, error) {
				if maxMembers != 6 {
					t.Fatalf("max members = %d, want 6", maxMembers)
				}

				return expected, nil
			},
		}

		_, err := newTestChats(t, repo, 6).Join(context.Background(), token, userID)
		if err != nil {
			t.Fatalf("join: %v", err)
		}
	})

	t.Run("rejects nil token", func(t *testing.T) {
		repo := &fakeRepo{
			joinFn: func(context.Context, uuid.UUID, uuid.UUID, int) (service.Chat, error) {
				t.Fatal("repository must not be called")

				return service.Chat{}, nil
			},
		}

		_, err := newTestChats(t, repo, 4).Join(context.Background(), uuid.Nil, userID)
		if !errors.Is(err, service.ErrInvalidToken) {
			t.Fatalf("error = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("maps errors", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
			want error
		}{
			{name: "not found", err: repository.ErrNotFound, want: service.ErrChatNotFound},
			{name: "banned", err: repository.ErrBanned, want: service.ErrBanned},
			{name: "full", err: repository.ErrFull, want: service.ErrChatFull},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := &fakeRepo{
					joinFn: func(context.Context, uuid.UUID, uuid.UUID, int) (service.Chat, error) {
						return service.Chat{}, tt.err
					},
				}

				_, err := newTestChats(t, repo, 4).Join(context.Background(), token, userID)
				if !errors.Is(err, tt.want) {
					t.Fatalf("error = %v, want %v", err, tt.want)
				}
			})
		}
	})

	t.Run("wraps unexpected error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			joinFn: func(context.Context, uuid.UUID, uuid.UUID, int) (service.Chat, error) {
				return service.Chat{}, dbErr
			},
		}

		_, err := newTestChats(t, repo, 4).Join(context.Background(), token, userID)
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestByID(t *testing.T) {
	expected := service.Chat{ID: uuid.New()}

	t.Run("returns chat", func(t *testing.T) {
		repo := &fakeRepo{
			getByIDFn: func(context.Context, uuid.UUID) (service.Chat, error) {
				return expected, nil
			},
		}

		chat, err := newTestChats(t, repo, 4).ByID(context.Background(), expected.ID)
		if err != nil {
			t.Fatalf("by id: %v", err)
		}
		if chat != expected {
			t.Fatalf("chat = %+v", chat)
		}
	})

	t.Run("maps not found", func(t *testing.T) {
		repo := &fakeRepo{
			getByIDFn: func(context.Context, uuid.UUID) (service.Chat, error) {
				return service.Chat{}, repository.ErrNotFound
			},
		}

		_, err := newTestChats(t, repo, 4).ByID(context.Background(), uuid.New())
		if !errors.Is(err, service.ErrChatNotFound) {
			t.Fatalf("error = %v, want ErrChatNotFound", err)
		}
	})
}

func TestListByUser(t *testing.T) {
	expected := []service.Chat{{ID: uuid.New()}, {ID: uuid.New()}}
	repo := &fakeRepo{
		listByUserFn: func(context.Context, uuid.UUID) ([]service.Chat, error) {
			return expected, nil
		},
	}

	chats, err := newTestChats(t, repo, 4).ListByUser(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(chats) != len(expected) {
		t.Fatalf("chats = %+v", chats)
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "ok", err: nil, want: nil},
		{name: "not found", err: repository.ErrNotFound, want: service.ErrChatNotFound},
		{name: "not host", err: repository.ErrNotHost, want: service.ErrNotHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{
				deleteFn: func(context.Context, uuid.UUID, uuid.UUID) error {
					return tt.err
				},
			}

			err := newTestChats(t, repo, 4).Delete(context.Background(), uuid.New(), uuid.New())
			if !errors.Is(err, tt.want) && (err != nil || tt.want != nil) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBan(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "ok", err: nil, want: nil},
		{name: "not found", err: repository.ErrNotFound, want: service.ErrChatNotFound},
		{name: "not host", err: repository.ErrNotHost, want: service.ErrNotHost},
		{name: "cannot ban host", err: repository.ErrCannotBanHost, want: service.ErrCannotBanHost},
		{name: "not member", err: repository.ErrNotMember, want: service.ErrUserNotMember},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{
				banFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
					return tt.err
				},
			}

			err := newTestChats(t, repo, 4).Ban(context.Background(), uuid.New(), uuid.New(), uuid.New())
			if !errors.Is(err, tt.want) && (err != nil || tt.want != nil) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestIsMemberAndMembers(t *testing.T) {
	chatID := uuid.New()
	userID := uuid.New()
	repo := &fakeRepo{
		isMemberFn: func(_ context.Context, gotChatID, gotUserID uuid.UUID) (bool, error) {
			if gotChatID != chatID || gotUserID != userID {
				t.Fatalf("got chat=%v user=%v", gotChatID, gotUserID)
			}

			return true, nil
		},
		membersFn: func(_ context.Context, gotChatID uuid.UUID) ([]uuid.UUID, error) {
			if gotChatID != chatID {
				t.Fatalf("got chat=%v", gotChatID)
			}

			return []uuid.UUID{userID}, nil
		},
	}

	chats := newTestChats(t, repo, 4)

	member, err := chats.IsMember(context.Background(), chatID, userID)
	if err != nil || !member {
		t.Fatalf("is member = %v, err = %v", member, err)
	}

	members, err := chats.Members(context.Background(), chatID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || members[0] != userID {
		t.Fatalf("members = %v", members)
	}
}
