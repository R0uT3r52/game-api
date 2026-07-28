package datasource

import (
	"game-api/internal/domain"
	"time"
)

func FromDomain(game *domain.Session) GameModel {
	var p1, p2, ct, w *string
	if game.Player1UUID != "" {
		p1 = &game.Player1UUID
	}
	if game.Player2UUID != "" {
		p2 = &game.Player2UUID
	}
	if game.CurrentTurnUUID != "" {
		ct = &game.CurrentTurnUUID
	}
	if game.WinnerUUID != "" {
		w = &game.WinnerUUID
	}

	return GameModel{
		UUID:            game.UUID,
		Field:           game.F.Grid,
		Player1UUID:     p1,
		Player2UUID:     p2,
		CurrentTurnUUID: ct,
		Status:          game.Status,
		IsWithBot:       game.IsWithBot,
		WinnerUUID:      w,
		Player1Sign:     game.Player1Sign,
		Player2Sign:     game.Player2Sign,
		ChangedAt:       time.Now(),
	}
}

func ToDomain(item *GameModel) *domain.Session {
	f := domain.Field{
		Grid: item.Field,
	}

	var p1, p2, ct, w string
	if item.Player1UUID != nil {
		p1 = *item.Player1UUID
	}
	if item.Player2UUID != nil {
		p2 = *item.Player2UUID
	}
	if item.CurrentTurnUUID != nil {
		ct = *item.CurrentTurnUUID
	}
	if item.WinnerUUID != nil {
		w = *item.WinnerUUID
	}

	return &domain.Session{
		UUID:            item.UUID,
		F:               f,
		Player1UUID:     p1,
		Player2UUID:     p2,
		CurrentTurnUUID: ct,
		Status:          item.Status,
		IsWithBot:       item.IsWithBot,
		WinnerUUID:      w,
		Player1Sign:     item.Player1Sign,
		Player2Sign:     item.Player2Sign,
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
