package datasource

import (
	"game-api/internal/domain"
	"time"
)

func FromDomain(game *domain.Session) GameModel {
	return GameModel{
		UUID:      game.UUID,
		Field:     game.F.Grid,
		ChangedAt: time.Now(),
	}
}

func ToDomain(item *GameModel) *domain.Session {
	f := domain.Field{
		Grid: item.Field,
	}

	return &domain.Session{
		UUID: item.UUID,
		F:    f,
	}
}

func UserToDomain(item *UserModel) *domain.User {
	return &domain.User{
		UUID:         item.UUID,
		Login:        item.Login,
		PasswordHash: item.PasswordHash,
	}
}

func UserFromDomain(item *domain.User) UserModel {
	return UserModel{
		UUID:         item.UUID,
		Login:        item.Login,
		PasswordHash: item.PasswordHash,
		CreatedAt:    time.Now(),
	}
}
