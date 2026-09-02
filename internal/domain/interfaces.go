package domain

import "context"

type GameRepositoryInterface interface {
	Save(ctx context.Context, game *Session) error
	Load(ctx context.Context, uuid string) (*Session, error)
	ListAvailable(ctx context.Context) ([]Session, error)
	GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]Session, error)
}

type GameServiceInterface interface {
	MakeAiMove(ctx context.Context, uuid string) (*Session, error)
	MakeMove(ctx context.Context, gameUUID, playerUUID string, newField Field) (*Session, error)
	ValidateField(ctx context.Context, uuid string, newField Field) error
	CheckGameEnd(ctx context.Context, uuid string) (isEnded bool, winner int, err error)
	CreateGame(ctx context.Context, p1 string, withBot bool) (id string, err error)
	GetAvailableGames(ctx context.Context) ([]Session, error)
	GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]Session, error)
	Connect(ctx context.Context, p2UUID, gameUUID string) error
}

type UserServiceInterface interface {
	GetUser(ctx context.Context, uuid string) (*User, error)
	GetUserByLogin(ctx context.Context, login string) (*User, error)
	SaveUser(ctx context.Context, u User) error
}

type AuthServiceInterface interface {
	Register(ctx context.Context, req SignUpRequest) error
	Authorize(ctx context.Context, login, password string) (uuid string, err error)
	GetUser(ctx context.Context, uuid string) (*User, error)
}
