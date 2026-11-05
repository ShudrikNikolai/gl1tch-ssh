package converter

import (
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/repository/models"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/messages/service"
)

func ToDomain(msg models.Message) service.Message {
	return service.Message{
		ID:        msg.ID,
		ChatID:    msg.ChatID,
		AuthorID:  msg.AuthorID,
		Text:      msg.Text,
		CreatedAt: msg.CreatedAt.UTC(),
	}
}

func ToModel(msg service.Message) models.Message {
	return models.Message{
		ID:        msg.ID,
		ChatID:    msg.ChatID,
		AuthorID:  msg.AuthorID,
		Text:      msg.Text,
		CreatedAt: msg.CreatedAt,
	}
}

func ToDomains(msgs []models.Message) []service.Message {
	result := make([]service.Message, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, ToDomain(msg))
	}

	return result
}
