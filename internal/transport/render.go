package transport

import (
	"fmt"

	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/aggregate"
	chatsvc "github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
	"github.com/google/uuid"
)

func formatMessage(msg aggregate.ChatMessage) string {
	return fmt.Sprintf("[%s] %s: %s\n", msg.CreatedAt.Format("15:04"), msg.AuthorNickname, msg.Text)
}

func formatChat(chat chatsvc.Chat, userID uuid.UUID) string {
	role := ""
	if chat.HostID == userID {
		role = " (host)"
	}

	return fmt.Sprintf(
		"- %s token=%s%s created=%s\n",
		chat.ID,
		chat.Token,
		role,
		chat.CreatedAt.Format("2006-01-02 15:04"),
	)
}
