package web

import "game-api/internal/domain"

type GameModel struct {
	UUID   string    `json:"uuid"`
	Field  [3][3]int `json:"field"`
	Winner *int      `json:"winner,omitempty"`
}

type GameHandler struct {
	Service domain.GameServiceInterface
}
