package aggregate

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

type fakeChats struct {
	isMemberFn func(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

func (f *fakeChats) Create(context.Context, uuid.UUID) (chatsvc.Chat, error) {
	return chatsvc.Chat{}, nil
}

func (f *fakeChats) Join(context.Context, uuid.UUID, uuid.UUID) (chatsvc.Chat, error) {
	return chatsvc.Chat{}, nil
}

func (f *fakeChats) ByID(context.Context, uuid.UUID) (chatsvc.Chat, error) {
	return chatsvc.Chat{}, nil
}

func (f *fakeChats) ListByUser(context.Context, uuid.UUID) ([]chatsvc.Chat, error) {
	return nil, nil
}

func (f *fakeChats) Delete(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakeChats) Ban(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }

func (f *fakeChats) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	if f.isMemberFn == nil {
		return false, nil
	}

	return f.isMemberFn(ctx, chatID, userID)
}

func (f *fakeChats) Members(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

type fakeUsers struct {
	byIDsFn func(context.Context, []uuid.UUID) (map[uuid.UUID]usersvc.User, error)
	calls   int
}

func (f *fakeUsers) Register(context.Context, string, string) (usersvc.User, error) {
	return usersvc.User{}, nil
}

func (f *fakeUsers) ByFingerprint(context.Context, string) (usersvc.User, error) {
	return usersvc.User{}, nil
}

func (f *fakeUsers) ByNickname(context.Context, string) (usersvc.User, error) {
	return usersvc.User{}, nil
}

func (f *fakeUsers) ByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]usersvc.User, error) {
	f.calls++
	if f.byIDsFn == nil {
		return map[uuid.UUID]usersvc.User{}, nil
	}

	return f.byIDsFn(ctx, ids)
}

type fakeMessages struct {
	sendFn    func(context.Context, uuid.UUID, uuid.UUID, string) (messagesvc.Message, error)
	historyFn func(context.Context, uuid.UUID, *messagesvc.Cursor, int) ([]messagesvc.Message, error)
}

func (f *fakeMessages) Send(
	ctx context.Context,
	chatID, authorID uuid.UUID,
	text string,
) (messagesvc.Message, error) {
	if f.sendFn == nil {
		return messagesvc.Message{}, nil
	}

	return f.sendFn(ctx, chatID, authorID, text)
}

func (f *fakeMessages) History(
	ctx context.Context,
	chatID uuid.UUID,
	before *messagesvc.Cursor,
	limit int,
) ([]messagesvc.Message, error) {
	if f.historyFn == nil {
		return nil, nil
	}

	return f.historyFn(ctx, chatID, before, limit)
}

type fakeBroadcaster struct {
	subscribeFn func(context.Context, uuid.UUID) (<-chan messagesvc.Message, func(), error)
}

func (f *fakeBroadcaster) Publish(context.Context, uuid.UUID, messagesvc.Message) error {
	return nil
}

func (f *fakeBroadcaster) Subscribe(
	ctx context.Context,
	chatID uuid.UUID,
) (<-chan messagesvc.Message, func(), error) {
	if f.subscribeFn == nil {
		return nil, nil, errors.New("not implemented")
	}

	return f.subscribeFn(ctx, chatID)
}

func newTestAggregate(
	chats chatsvc.Service,
	messages messagesvc.Service,
	users usersvc.Service,
	broadcaster messagesvc.Broadcaster,
) *Aggregate {
	return New(
		chats,
		messages,
		users,
		broadcaster,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func TestSendMessage(t *testing.T) {
	chatID := uuid.New()
	authorID := uuid.New()
	msg := messagesvc.Message{
		ID:        uuid.New(),
		ChatID:    chatID,
		AuthorID:  authorID,
		Text:      "hello",
		CreatedAt: time.Now().UTC(),
	}

	t.Run("returns enriched message", func(t *testing.T) {
		chats := &fakeChats{
			isMemberFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		messages := &fakeMessages{
			sendFn: func(
				_ context.Context,
				gotChatID, gotAuthorID uuid.UUID,
				text string,
			) (messagesvc.Message, error) {
				if gotChatID != chatID || gotAuthorID != authorID || text != "hello" {
					t.Fatalf("got chat=%v author=%v text=%q", gotChatID, gotAuthorID, text)
				}

				return msg, nil
			},
		}
		users := &fakeUsers{
			byIDsFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
				return map[uuid.UUID]usersvc.User{
					authorID: {ID: authorID, Nickname: "alice"},
				}, nil
			},
		}

		got, err := newTestAggregate(chats, messages, users, &fakeBroadcaster{}).
			SendMessage(context.Background(), chatID, authorID, "hello")
		if err != nil {
			t.Fatalf("send message: %v", err)
		}
		if got.AuthorNickname != "alice" || got.Text != "hello" || got.ID != msg.ID {
			t.Fatalf("message = %+v", got)
		}
	})

	t.Run("rejects non-member", func(t *testing.T) {
		chats := &fakeChats{
			isMemberFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
				return false, nil
			},
		}
		messages := &fakeMessages{
			sendFn: func(context.Context, uuid.UUID, uuid.UUID, string) (messagesvc.Message, error) {
				t.Fatal("messages service must not be called")

				return messagesvc.Message{}, nil
			},
		}

		_, err := newTestAggregate(chats, messages, &fakeUsers{}, &fakeBroadcaster{}).
			SendMessage(context.Background(), chatID, authorID, "hello")
		if !errors.Is(err, ErrNotInChat) {
			t.Fatalf("error = %v, want ErrNotInChat", err)
		}
	})

	t.Run("wraps membership error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		chats := &fakeChats{
			isMemberFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
				return false, dbErr
			},
		}

		_, err := newTestAggregate(chats, &fakeMessages{}, &fakeUsers{}, &fakeBroadcaster{}).
			SendMessage(context.Background(), chatID, authorID, "hello")
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})

	t.Run("wraps send error", func(t *testing.T) {
		sendErr := messagesvc.ErrMessageTooLong
		chats := &fakeChats{
			isMemberFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		messages := &fakeMessages{
			sendFn: func(context.Context, uuid.UUID, uuid.UUID, string) (messagesvc.Message, error) {
				return messagesvc.Message{}, sendErr
			},
		}
		users := &fakeUsers{
			byIDsFn: func(context.Context, []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
				t.Fatal("users service must not be called")

				return nil, nil
			},
		}

		_, err := newTestAggregate(chats, messages, users, &fakeBroadcaster{}).
			SendMessage(context.Background(), chatID, authorID, "hello")
		if !errors.Is(err, messagesvc.ErrMessageTooLong) {
			t.Fatalf("error = %v, want ErrMessageTooLong", err)
		}
	})

	t.Run("falls back on unknown author", func(t *testing.T) {
		chats := &fakeChats{
			isMemberFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		messages := &fakeMessages{
			sendFn: func(context.Context, uuid.UUID, uuid.UUID, string) (messagesvc.Message, error) {
				return msg, nil
			},
		}

		got, err := newTestAggregate(chats, messages, &fakeUsers{}, &fakeBroadcaster{}).
			SendMessage(context.Background(), chatID, authorID, "hello")
		if err != nil {
			t.Fatalf("send message: %v", err)
		}
		if got.AuthorNickname != unknownNickname {
			t.Fatalf("nickname = %q, want %q", got.AuthorNickname, unknownNickname)
		}
	})
}

func TestHistory(t *testing.T) {
	chatID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()
	base := time.Now().UTC()
	messages := []messagesvc.Message{
		{ID: uuid.New(), ChatID: chatID, AuthorID: aliceID, Text: "one", CreatedAt: base},
		{ID: uuid.New(), ChatID: chatID, AuthorID: bobID, Text: "two", CreatedAt: base.Add(time.Second)},
		{ID: uuid.New(), ChatID: chatID, AuthorID: aliceID, Text: "three", CreatedAt: base.Add(2 * time.Second)},
	}

	t.Run("enriches unique authors", func(t *testing.T) {
		var gotIDs []uuid.UUID

		users := &fakeUsers{
			byIDsFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
				gotIDs = ids

				return map[uuid.UUID]usersvc.User{
					aliceID: {ID: aliceID, Nickname: "alice"},
					bobID:   {ID: bobID, Nickname: "bob"},
				}, nil
			},
		}
		messagesSrv := &fakeMessages{
			historyFn: func(
				context.Context,
				uuid.UUID,
				*messagesvc.Cursor,
				int,
			) ([]messagesvc.Message, error) {
				return messages, nil
			},
		}

		result, err := newTestAggregate(&fakeChats{}, messagesSrv, users, &fakeBroadcaster{}).
			History(context.Background(), chatID, nil, 20)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		if len(result) != 3 {
			t.Fatalf("result = %+v", result)
		}
		if result[0].AuthorNickname != "alice" || result[1].AuthorNickname != "bob" {
			t.Fatalf("result = %+v", result)
		}
		if len(gotIDs) != 2 {
			t.Fatalf("author ids = %v, want 2 unique", gotIDs)
		}
	})

	t.Run("empty history skips users", func(t *testing.T) {
		users := &fakeUsers{
			byIDsFn: func(context.Context, []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
				t.Fatal("users service must not be called")

				return nil, nil
			},
		}
		messagesSrv := &fakeMessages{
			historyFn: func(
				context.Context,
				uuid.UUID,
				*messagesvc.Cursor,
				int,
			) ([]messagesvc.Message, error) {
				return nil, nil
			},
		}

		result, err := newTestAggregate(&fakeChats{}, messagesSrv, users, &fakeBroadcaster{}).
			History(context.Background(), chatID, nil, 20)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("falls back when users fail", func(t *testing.T) {
		users := &fakeUsers{
			byIDsFn: func(context.Context, []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
				return nil, errors.New("db is down")
			},
		}
		messagesSrv := &fakeMessages{
			historyFn: func(
				context.Context,
				uuid.UUID,
				*messagesvc.Cursor,
				int,
			) ([]messagesvc.Message, error) {
				return messages, nil
			},
		}

		result, err := newTestAggregate(&fakeChats{}, messagesSrv, users, &fakeBroadcaster{}).
			History(context.Background(), chatID, nil, 20)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		if result[0].AuthorNickname != unknownNickname {
			t.Fatalf("nickname = %q", result[0].AuthorNickname)
		}
	})

	t.Run("wraps history error", func(t *testing.T) {
		dbErr := errors.New("db is down")
		messagesSrv := &fakeMessages{
			historyFn: func(
				context.Context,
				uuid.UUID,
				*messagesvc.Cursor,
				int,
			) ([]messagesvc.Message, error) {
				return nil, dbErr
			},
		}

		_, err := newTestAggregate(&fakeChats{}, messagesSrv, &fakeUsers{}, &fakeBroadcaster{}).
			History(context.Background(), chatID, nil, 20)
		if !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want wrapped db error", err)
		}
	})
}

func TestSubscribe(t *testing.T) {
	chatID := uuid.New()
	aliceID := uuid.New()
	bobID := uuid.New()
	raw := make(chan messagesvc.Message)

	var closeOnce sync.Once
	rawUnsubscribe := func() {
		closeOnce.Do(func() { close(raw) })
	}

	users := &fakeUsers{
		byIDsFn: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]usersvc.User, error) {
			result := make(map[uuid.UUID]usersvc.User, len(ids))
			for _, id := range ids {
				switch id {
				case aliceID:
					result[id] = usersvc.User{ID: aliceID, Nickname: "alice"}
				case bobID:
					result[id] = usersvc.User{ID: bobID, Nickname: "bob"}
				}
			}

			return result, nil
		},
	}

	broadcaster := &fakeBroadcaster{
		subscribeFn: func(context.Context, uuid.UUID) (<-chan messagesvc.Message, func(), error) {
			return raw, rawUnsubscribe, nil
		},
	}

	agg := newTestAggregate(&fakeChats{}, &fakeMessages{}, users, broadcaster)

	messages, unsubscribe, err := agg.Subscribe(context.Background(), chatID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	send := func(authorID uuid.UUID, text string) ChatMessage {
		t.Helper()

		raw <- messagesvc.Message{
			ID:        uuid.New(),
			ChatID:    chatID,
			AuthorID:  authorID,
			Text:      text,
			CreatedAt: time.Now().UTC(),
		}

		select {
		case msg := <-messages:
			return msg
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for message")

			return ChatMessage{}
		}
	}

	if got := send(aliceID, "one"); got.AuthorNickname != "alice" {
		t.Fatalf("message = %+v", got)
	}
	if got := send(aliceID, "two"); got.AuthorNickname != "alice" {
		t.Fatalf("message = %+v", got)
	}
	if users.calls != 1 {
		t.Fatalf("user lookups = %d, want 1 (cache)", users.calls)
	}
	if got := send(bobID, "three"); got.AuthorNickname != "bob" {
		t.Fatalf("message = %+v", got)
	}
	if users.calls != 2 {
		t.Fatalf("user lookups = %d, want 2", users.calls)
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

func TestSubscribeError(t *testing.T) {
	subErr := errors.New("redis is down")
	broadcaster := &fakeBroadcaster{
		subscribeFn: func(context.Context, uuid.UUID) (<-chan messagesvc.Message, func(), error) {
			return nil, nil, subErr
		},
	}

	_, _, err := newTestAggregate(&fakeChats{}, &fakeMessages{}, &fakeUsers{}, broadcaster).
		Subscribe(context.Background(), uuid.New())
	if !errors.Is(err, subErr) {
		t.Fatalf("error = %v, want wrapped subscribe error", err)
	}
}
