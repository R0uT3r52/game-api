package web

import "game-api/internal/domain"

func (g *GameModel) WebToDomain() *domain.Session {
	return &domain.Session{
		UUID: g.UUID,
		F: domain.Field{
			Grid: g.Field,
		},
		Player1UUID:     g.Player1UUID,
		Player2UUID:     g.Player2UUID,
		CurrentTurnUUID: g.CurrentTurnUUID,
		Status:          g.Status,
		IsWithBot:       g.IsWithBot,
		WinnerUUID:      g.WinnerUUID,
		Player1Sign:     g.Player1Sign,
		Player2Sign:     g.Player2Sign,
	}
}

func (g *GameModel) DomainToWeb(s *domain.Session) {
	g.UUID = s.UUID
	g.Field = s.F.Grid
	g.Player1UUID = s.Player1UUID
	g.Player2UUID = s.Player2UUID
	g.CurrentTurnUUID = s.CurrentTurnUUID
	g.Status = s.Status
	g.IsWithBot = s.IsWithBot
	g.WinnerUUID = s.WinnerUUID
	g.Player1Sign = s.Player1Sign
	g.Player2Sign = s.Player2Sign
}
