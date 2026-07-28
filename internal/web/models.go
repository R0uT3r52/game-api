package web

import "game-api/internal/domain"

type GameModel struct {
	UUID            string    `json:"uuid"`
	Field           [3][3]int `json:"field"`
	Player1UUID     string    `json:"player1_uuid"`
	Player2UUID     string    `json:"player2_uuid,omitempty"`
	CurrentTurnUUID string    `json:"current_turn_uuid,omitempty"`
	Status          int       `json:"status"`
	IsWithBot       bool      `json:"is_with_bot"`
	WinnerUUID      string    `json:"winner_uuid,omitempty"`
	Player1Sign     int       `json:"player1_sign"`
	Player2Sign     int       `json:"player2_sign"`
}

type CreateGameRequest struct {
	IsWithBot bool `json:"is_with_bot"`
}

type ConnectGameRequest struct {
	GameUUID string `json:"uuid"`
}

type MoveRequest struct {
	Field [3][3]int `json:"field"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type UserResponse struct {
	UUID  string `json:"uuid"`
	Login string `json:"login"`
}

type TokenResponse struct {
	UUID string `json:"uuid"`
}

type UserHandler struct {
	Service domain.AuthServiceInterface
}

type UserAuthenticator struct {
	AuthService domain.AuthServiceInterface
}

type GameHandler struct {
	Service domain.GameServiceInterface
}
