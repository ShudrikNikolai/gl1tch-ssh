package converter

import (
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/repository/models"
	"github.com/ShudrikNikolai/gl1tch-ssh/internal/modules/users/service"
)

func ToDomain(user models.User) service.User {
	return service.User{
		ID:          user.ID,
		Nickname:    user.Nickname,
		Fingerprint: user.Fingerprint,
		CreatedAt:   user.CreatedAt,
	}
}

func ToModel(user service.User) models.User {
	return models.User{
		ID:          user.ID,
		Nickname:    user.Nickname,
		Fingerprint: user.Fingerprint,
		CreatedAt:   user.CreatedAt,
	}
}
