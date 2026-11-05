package srv

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
	"github.com/google/uuid"
)

const defaultMaxRunes = 300

type Messages struct {
	repo        repository.Repository
	broadcaster service.Broadcaster
	log         *slog.Logger
	maxRunes    int
}

func New(
	repo repository.Repository,
	broadcaster service.Broadcaster,
	log *slog.Logger,
	maxRunes int,
) *Messages {
	if maxRunes <= 0 {
		maxRunes = defaultMaxRunes
	}

	return &Messages{
		repo:        repo,
		broadcaster: broadcaster,
		log:         log,
		maxRunes:    maxRunes,
	}
}

func (m *Messages) Send(
	ctx context.Context,
	chatID, authorID uuid.UUID,
	text string,
) (service.Message, error) {
	text = sanitizeText(text)
	if text == "" {
		return service.Message{}, service.ErrEmptyMessage
	}
	if utf8.RuneCountInString(text) > m.maxRunes {
		return service.Message{}, service.ErrMessageTooLong
	}

	id, err := uuid.NewV7()
	if err != nil {
		return service.Message{}, fmt.Errorf("generate message id: %w", err)
	}

	msg := service.Message{
		ID:        id,
		ChatID:    chatID,
		AuthorID:  authorID,
		Text:      text,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	if err = m.repo.Insert(ctx, msg); err != nil {
		return service.Message{}, fmt.Errorf("insert message: %w", err)
	}

	if err = m.broadcaster.Publish(ctx, chatID, msg); err != nil {
		m.log.Error("failed to publish message", "error", err, "chat_id", chatID, "message_id", id)
	}

	return msg, nil
}

func (m *Messages) History(
	ctx context.Context,
	chatID uuid.UUID,
	before *service.Cursor,
	limit int,
) ([]service.Message, error) {
	limit = normalizeHistoryLimit(limit)

	var (
		messages []service.Message
		err      error
	)

	if before == nil {
		messages, err = m.repo.ListLatest(ctx, chatID, limit)
	} else {
		messages, err = m.repo.ListBefore(ctx, chatID, *before, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	return messages, nil
}

func normalizeHistoryLimit(limit int) int {
	if limit <= 0 {
		return service.DefaultHistoryLimit
	}
	if limit > service.MaxHistoryLimit {
		return service.MaxHistoryLimit
	}

	return limit
}

// sanitizeText убирает управляющие символы, чтобы чужой текст не мог
// инжектить ANSI-последовательности в терминалы участников.
func sanitizeText(text string) string {
	var builder strings.Builder

	builder.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			builder.WriteRune(' ')
		case unicode.IsControl(r):
			continue
		default:
			builder.WriteRune(r)
		}
	}

	return strings.TrimSpace(builder.String())
}
