package srv

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/google/uuid"
)

type fakeRepo struct {
	insertFn     func(context.Context, service.Message) error
	listLatestFn func(context.Context, uuid.UUID, int) ([]service.Message, error)
	listBeforeFn func(context.Context, uuid.UUID, service.Cursor, int) ([]service.Message, error)
}

func (f *fakeRepo) Insert(ctx context.Context, msg service.Message) error {
	if f.insertFn == nil {
		return nil
	}

	return f.insertFn(ctx, msg)
}

func (f *fakeRepo) ListLatest(ctx context.Context, chatID uuid.UUID, limit int) ([]service.Message, error) {
	if f.listLatestFn == nil {
		return nil, nil
	}

	return f.listLatestFn(ctx, chatID, limit)
}

func (f *fakeRepo) ListBefore(
	ctx context.Context,
	chatID uuid.UUID,
	before service.Cursor,
	limit int,
) ([]service.Message, error) {
	if f.listBeforeFn == nil {
		return nil, nil
	}

	return f.listBeforeFn(ctx, chatID, before, limit)
}

type fakeBroadcaster struct {
	publishFn func(context.Context, uuid.UUID, service.Message) error
}

func (f *fakeBroadcaster) Publish(ctx context.Context, chatID uuid.UUID, msg service.Message) error {
	if f.publishFn == nil {
		return nil
	}

	return f.publishFn(ctx, chatID, msg)
}

func (f *fakeBroadcaster) Subscribe(context.Context, uuid.UUID) (<-chan service.Message, func(), error) {
	return nil, nil, errors.New("not implemented")
}

func newTestMessages(t *testing.T, repo repository.Repository, broadcaster service.Broadcaster, maxRunes int) *Messages {
	t.Helper()

	return New(repo, broadcaster, slog.New(slog.NewTextHandler(io.Discard, nil)), maxRunes)
}

func TestSend(t *testing.T) {
	chatID := uuid.New()
	authorID := uuid.New()

	t.Run("saves and publishes message", func(t *testing.T) {
		var (
			saved     service.Message
			published service.Message
		)

		repo := &fakeRepo{
			insertFn: func(_ context.Context, msg service.Message) error {
				saved = msg

				return nil
			},
		}
		broadcaster := &fakeBroadcaster{
			publishFn: func(_ context.Context, gotChatID uuid.UUID, msg service.Message) error {
				if gotChatID != chatID {
					t.Fatalf("chat id = %v, want %v", gotChatID, chatID)
				}
				published = msg

				return nil
			},
		}

		msg, err := newTestMessages(t, repo, broadcaster, 300).Send(context.Background(), chatID, authorID, "  hello  ")
		if err != nil {
			t.Fatalf("send: %v", err)
		}

		if msg.ID == uuid.Nil || msg.ID.Version() != 7 {
			t.Fatalf("message id = %v, want uuidv7", msg.ID)
		}
		if msg.Text != "hello" {
			t.Fatalf("text = %q, want hello", msg.Text)
		}
		if msg.ChatID != chatID || msg.AuthorID != authorID {
			t.Fatalf("message = %+v", msg)
		}
		if msg.CreatedAt.IsZero() {
			t.Fatal("created_at is zero")
		}
		if saved != msg || published != msg {
			t.Fatalf("saved=%+v published=%+v msg=%+v", saved, published, msg)
		}
	})

	t.Run("sanitizes control characters", func(t *testing.T) {
		var saved service.Message

		repo := &fakeRepo{
			insertFn: func(_ context.Context, msg service.Message) error {
				saved = msg

				return nil
			},
		}

		msg, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).Send(
			context.Background(),
			chatID,
			authorID,
			"\x1b[31mhello\x1b[0m\n",
		)
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if msg.Text != "[31mhello[0m" {
			t.Fatalf("text = %q, want control chars removed", msg.Text)
		}
		if saved.Text != msg.Text {
			t.Fatalf("saved text = %q", saved.Text)
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		tests := []string{"", "   ", "\x1b", "\n\t"}

		for _, text := range tests {
			repo := &fakeRepo{
				insertFn: func(context.Context, service.Message) error {
					t.Fatal("repository must not be called")

					return nil
				},
			}

			_, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).Send(
				context.Background(), chatID, authorID, text,
			)
			if !errors.Is(err, service.ErrEmptyMessage) {
				t.Fatalf("text %q: error = %v, want ErrEmptyMessage", text, err)
			}
		}
	})

	t.Run("rejects long message", func(t *testing.T) {
		repo := &fakeRepo{
			insertFn: func(context.Context, service.Message) error {
				t.Fatal("repository must not be called")

				return nil
			},
		}

		_, err := newTestMessages(t, repo, &fakeBroadcaster{}, 5).Send(
			context.Background(), chatID, authorID, "123456",
		)
		if !errors.Is(err, service.ErrMessageTooLong) {
			t.Fatalf("error = %v, want ErrMessageTooLong", err)
		}

		if _, err = newTestMessages(t, &fakeRepo{}, &fakeBroadcaster{}, 5).Send(
			context.Background(), chatID, authorID, "12345",
		); err != nil {
			t.Fatalf("exact limit must pass: %v", err)
		}
	})

	t.Run("wraps insert error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			insertFn: func(context.Context, service.Message) error {
				return dbErr
			},
		}
		broadcaster := &fakeBroadcaster{
			publishFn: func(context.Context, uuid.UUID, service.Message) error {
				t.Fatal("broadcaster must not be called")

				return nil
			},
		}

		_, err := newTestMessages(t, repo, broadcaster, 300).Send(
			context.Background(), chatID, authorID, "hello",
		)
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})

	t.Run("keeps message when broadcast fails", func(t *testing.T) {
		repo := &fakeRepo{}
		broadcaster := &fakeBroadcaster{
			publishFn: func(context.Context, uuid.UUID, service.Message) error {
				return errors.New("redis is down")
			},
		}

		msg, err := newTestMessages(t, repo, broadcaster, 300).Send(
			context.Background(), chatID, authorID, "hello",
		)
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if msg.Text != "hello" {
			t.Fatalf("text = %q", msg.Text)
		}
	})
}

func TestHistory(t *testing.T) {
	chatID := uuid.New()
	latest := []service.Message{{ID: uuid.New(), Text: "latest"}}
	older := []service.Message{{ID: uuid.New(), Text: "older"}}

	t.Run("uses latest without cursor", func(t *testing.T) {
		repo := &fakeRepo{
			listLatestFn: func(_ context.Context, gotChatID uuid.UUID, limit int) ([]service.Message, error) {
				if gotChatID != chatID || limit != service.DefaultHistoryLimit {
					t.Fatalf("chat=%v limit=%d", gotChatID, limit)
				}

				return latest, nil
			},
		}

		messages, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).History(
			context.Background(), chatID, nil, 0,
		)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		if len(messages) != 1 || messages[0] != latest[0] {
			t.Fatalf("messages = %+v", messages)
		}
	})

	t.Run("uses cursor", func(t *testing.T) {
		cursor := service.Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}
		repo := &fakeRepo{
			listBeforeFn: func(
				_ context.Context,
				gotChatID uuid.UUID,
				before service.Cursor,
				limit int,
			) ([]service.Message, error) {
				if gotChatID != chatID || before != cursor || limit != 5 {
					t.Fatalf("chat=%v before=%+v limit=%d", gotChatID, before, limit)
				}

				return older, nil
			},
		}

		messages, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).History(
			context.Background(), chatID, &cursor, 5,
		)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		if len(messages) != 1 || messages[0] != older[0] {
			t.Fatalf("messages = %+v", messages)
		}
	})

	t.Run("caps limit", func(t *testing.T) {
		repo := &fakeRepo{
			listLatestFn: func(_ context.Context, _ uuid.UUID, limit int) ([]service.Message, error) {
				if limit != service.MaxHistoryLimit {
					t.Fatalf("limit = %d, want %d", limit, service.MaxHistoryLimit)
				}

				return nil, nil
			},
		}

		if _, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).History(
			context.Background(), chatID, nil, 1000,
		); err != nil {
			t.Fatalf("history: %v", err)
		}
	})

	t.Run("wraps repository error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		repo := &fakeRepo{
			listLatestFn: func(context.Context, uuid.UUID, int) ([]service.Message, error) {
				return nil, dbErr
			},
		}

		_, err := newTestMessages(t, repo, &fakeBroadcaster{}, 300).History(
			context.Background(), chatID, nil, 20,
		)
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestNormalizeHistoryLimit(t *testing.T) {
	tests := []struct {
		limit int
		want  int
	}{
		{limit: -1, want: service.DefaultHistoryLimit},
		{limit: 0, want: service.DefaultHistoryLimit},
		{limit: 1, want: 1},
		{limit: service.MaxHistoryLimit, want: service.MaxHistoryLimit},
		{limit: service.MaxHistoryLimit + 1, want: service.MaxHistoryLimit},
	}

	for _, tt := range tests {
		if got := normalizeHistoryLimit(tt.limit); got != tt.want {
			t.Fatalf("normalizeHistoryLimit(%d) = %d, want %d", tt.limit, got, tt.want)
		}
	}
}

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "trim", text: "  hello  ", want: "hello"},
		{name: "ansi", text: "\x1b[31mred\x1b[0m", want: "[31mred[0m"},
		{name: "newline", text: "hello\nworld", want: "hello world"},
		{name: "tab", text: "hello\tworld", want: "hello world"},
		{name: "empty", text: strings.Repeat("\x00", 3), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeText(tt.text); got != tt.want {
				t.Fatalf("sanitizeText(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
