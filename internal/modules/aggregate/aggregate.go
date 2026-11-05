package aggregate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	messagesvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	usersvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
	"github.com/google/uuid"
)

var ErrNotInChat = errors.New("user is not in chat")

const unknownNickname = "unknown"

// ChatMessage — сообщение, обогащённое ником автора для транспорта.
type ChatMessage struct {
	ID             uuid.UUID
	ChatID         uuid.UUID
	AuthorID       uuid.UUID
	AuthorNickname string
	Text           string
	CreatedAt      time.Time
}

type Aggregate struct {
	chats       chatsvc.Service
	messages    messagesvc.Service
	users       usersvc.Service
	broadcaster messagesvc.Broadcaster
	log         *slog.Logger
}

func New(
	chats chatsvc.Service,
	messages messagesvc.Service,
	users usersvc.Service,
	broadcaster messagesvc.Broadcaster,
	log *slog.Logger,
) *Aggregate {
	return &Aggregate{
		chats:       chats,
		messages:    messages,
		users:       users,
		broadcaster: broadcaster,
		log:         log,
	}
}

func (a *Aggregate) SendMessage(
	ctx context.Context,
	chatID, authorID uuid.UUID,
	text string,
) (ChatMessage, error) {
	member, err := a.chats.IsMember(ctx, chatID, authorID)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("check chat membership: %w", err)
	}
	if !member {
		return ChatMessage{}, ErrNotInChat
	}

	msg, err := a.messages.Send(ctx, chatID, authorID, text)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("send message: %w", err)
	}

	nickname, err := a.nickname(ctx, make(map[uuid.UUID]string), authorID)
	if err != nil {
		a.log.Warn("failed to resolve author nickname", "error", err, "author_id", authorID)
		nickname = unknownNickname
	}

	return toChatMessage(msg, nickname), nil
}

func (a *Aggregate) History(
	ctx context.Context,
	chatID uuid.UUID,
	before *messagesvc.Cursor,
	limit int,
) ([]ChatMessage, error) {
	messages, err := a.messages.History(ctx, chatID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("get message history: %w", err)
	}
	if len(messages) == 0 {
		return []ChatMessage{}, nil
	}

	users, err := a.users.ByIDs(ctx, uniqueAuthorIDs(messages))
	if err != nil {
		a.log.Warn("failed to load message authors", "error", err, "chat_id", chatID)
		users = nil
	}

	result := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		nickname := unknownNickname
		if user, ok := users[msg.AuthorID]; ok {
			nickname = user.Nickname
		}
		result = append(result, toChatMessage(msg, nickname))
	}

	return result, nil
}

func (a *Aggregate) Subscribe(
	ctx context.Context,
	chatID uuid.UUID,
) (<-chan ChatMessage, func(), error) {
	raw, rawUnsubscribe, err := a.broadcaster.Subscribe(ctx, chatID)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribe to chat: %w", err)
	}

	out := make(chan ChatMessage)
	done := make(chan struct{})

	var once sync.Once

	unsubscribe := func() {
		once.Do(func() {
			close(done)
			rawUnsubscribe()
		})
	}

	go func() {
		defer close(out)

		cache := make(map[uuid.UUID]string)

		for {
			select {
			case <-done:
				return
			case msg, ok := <-raw:
				if !ok {
					return
				}

				nickname, err := a.nickname(ctx, cache, msg.AuthorID)
				if err != nil {
					a.log.Warn("failed to resolve author nickname", "error", err, "author_id", msg.AuthorID)
					nickname = unknownNickname
				}

				select {
				case out <- toChatMessage(msg, nickname):
				case <-done:
					return
				}
			}
		}
	}()

	return out, unsubscribe, nil
}

func (a *Aggregate) nickname(
	ctx context.Context,
	cache map[uuid.UUID]string,
	userID uuid.UUID,
) (string, error) {
	if nickname, ok := cache[userID]; ok {
		return nickname, nil
	}

	users, err := a.users.ByIDs(ctx, []uuid.UUID{userID})
	if err != nil {
		return "", fmt.Errorf("get author: %w", err)
	}

	user, ok := users[userID]
	if !ok {
		return "", fmt.Errorf("author %s not found", userID)
	}

	cache[userID] = user.Nickname

	return user.Nickname, nil
}

func uniqueAuthorIDs(messages []messagesvc.Message) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(messages))
	ids := make([]uuid.UUID, 0, len(messages))

	for _, msg := range messages {
		if _, ok := seen[msg.AuthorID]; ok {
			continue
		}
		seen[msg.AuthorID] = struct{}{}
		ids = append(ids, msg.AuthorID)
	}

	return ids
}

func toChatMessage(msg messagesvc.Message, nickname string) ChatMessage {
	return ChatMessage{
		ID:             msg.ID,
		ChatID:         msg.ChatID,
		AuthorID:       msg.AuthorID,
		AuthorNickname: nickname,
		Text:           msg.Text,
		CreatedAt:      msg.CreatedAt,
	}
}
