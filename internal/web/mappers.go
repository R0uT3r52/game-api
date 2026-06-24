package web

import "game-api/internal/domain"

func (g *GameModel) WebToDomain() *domain.Session {
	return &domain.Session{
		UUID: g.UUID,
		F: domain.Field{
			Grid: g.Field,
		},
	}
}

func (g *GameModel) DomainToWeb(s *domain.Session) {
	g.UUID = s.UUID
	g.Field = s.F.Grid
	g.Winner = nil
}
