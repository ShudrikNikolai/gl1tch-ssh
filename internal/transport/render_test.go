package transport

import (
	"strings"
	"testing"
	"time"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/aggregate"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
)

func TestFormatMessage(t *testing.T) {
	msg := aggregate.ChatMessage{
		AuthorNickname: "alice",
		Text:           "hello",
		CreatedAt:      time.Date(2026, 11, 5, 12, 34, 0, 0, time.UTC),
	}

	if got, want := formatMessage(msg), "[12:34] alice: hello\n"; got != want {
		t.Fatalf("formatMessage() = %q, want %q", got, want)
	}
}

func TestFormatChat(t *testing.T) {
	userID := uuid.New()
	chat := chatsvc.Chat{
		ID:        uuid.New(),
		HostID:    userID,
		Token:     uuid.New(),
		CreatedAt: time.Date(2026, 11, 5, 12, 34, 0, 0, time.UTC),
	}

	line := formatChat(chat, userID)
	if !strings.Contains(line, chat.Token.String()) || !strings.Contains(line, "(host)") {
		t.Fatalf("host line = %q", line)
	}

	other := formatChat(chat, uuid.New())
	if strings.Contains(other, "(host)") {
		t.Fatalf("non-host line = %q", other)
	}
}
