package redis

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestBroadcaster(t *testing.T) *RedisBroadcaster {
	t.Helper()

	mr := miniredis.RunT(t)
	pool := NewPool(mr.Addr())
	t.Cleanup(func() { _ = pool.Close() })

	return NewRedisBroadcaster(pool, testLogger())
}

func TestPublishSubscribe(t *testing.T) {
	b := newTestBroadcaster(t)
	chatID := uuid.New()
	ctx := context.Background()

	messages, unsubscribe, err := b.Subscribe(ctx, chatID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()

	want := messagesvc.Message{
		ID:        uuid.New(),
		ChatID:    chatID,
		AuthorID:  uuid.New(),
		Text:      "hello",
		CreatedAt: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC),
	}

	if err = b.Publish(ctx, chatID, want); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-messages:
		if got != want {
			t.Fatalf("message = %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := newTestBroadcaster(t)
	ctx := context.Background()

	messages, unsubscribe, err := b.Subscribe(ctx, uuid.New())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	unsubscribe()
	unsubscribe()

	select {
	case _, ok := <-messages:
		if ok {
			t.Fatal("channel must be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("channel was not closed")
	}
}

func TestContextCancelClosesChannel(t *testing.T) {
	b := newTestBroadcaster(t)
	ctx, cancel := context.WithCancel(context.Background())

	messages, _, err := b.Subscribe(ctx, uuid.New())
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	cancel()

	select {
	case _, ok := <-messages:
		if ok {
			t.Fatal("channel must be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("channel was not closed after context cancel")
	}
}

func TestPublishRejectsCanceledContext(t *testing.T) {
	b := newTestBroadcaster(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := b.Publish(ctx, uuid.New(), messagesvc.Message{}); err == nil {
		t.Fatal("publish must fail on canceled context")
	}
}
