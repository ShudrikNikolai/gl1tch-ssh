package converter

import (
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/repository/models"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/chats/service"
)

func ToDomain(chat models.Chat) service.Chat {
	return service.Chat{
		ID:        chat.ID,
		HostID:    chat.HostID,
		Token:     chat.Token,
		CreatedAt: chat.CreatedAt,
		DeletedAt: chat.DeletedAt,
	}
}

func ToModel(chat service.Chat) models.Chat {
	return models.Chat{
		ID:        chat.ID,
		HostID:    chat.HostID,
		Token:     chat.Token,
		CreatedAt: chat.CreatedAt,
		DeletedAt: chat.DeletedAt,
	}
}

func ToDomains(chats []models.Chat) []service.Chat {
	result := make([]service.Chat, 0, len(chats))
	for _, chat := range chats {
		result = append(result, ToDomain(chat))
	}

	return result
}
