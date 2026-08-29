package domain

import "sync"

// Cell states
const (
	Empty = iota
	Cross
	Nought
)

// Game states (FSM)
const (
	Waiting = iota
	Turn
	Draw
	Win
)

// Win states
const (
	Player = iota
	Bot
	Tie
)

type Field struct {
	Grid [3][3]int
}

type Session struct {
	UUID string
	F    Field

	Player1UUID     string
	Player2UUID     string
	CurrentTurnUUID string // Player UUID which turn it is

	Status    int
	IsWithBot bool

	WinnerUUID  string
	Player1Sign int
	Player2Sign int
}

type GameService struct {
	repo       GameRepositoryInterface
	PlayerSign int
	BotSign    int
	mu         sync.Mutex
}

type SignUpRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type User struct {
	UUID         string
	Login        string
	PasswordHash string
}

type AuthService struct {
	UserSvc UserServiceInterface
}
