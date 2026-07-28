package domain

type GameRepositoryInterface interface {
	Save(game *Session) error
	Load(uuid string) (*Session, error)
	ListAvailable() ([]Session, error)
	GetCurrentGames(gameUUID, playerUUID string) ([]Session, error)
}

type GameServiceInterface interface {
	MakeAiMove(uuid string) (*Session, error)
	MakeMove(gameUUID, playerUUID string, newField Field) (*Session, error)
	ValidateField(uuid string, newField Field) error
	CheckGameEnd(uuid string) (isEnded bool, winner int, err error)
	CreateGame(p1 string, withBot bool) (id string, err error)
	GetAvailableGames() ([]Session, error)
	GetCurrentGames(gameUUID, playerUUID string) ([]Session, error)
	Connect(p2UUID, gameUUID string) error
}

type UserServiceInterface interface {
	GetUser(uuid string) (*User, error)
	GetUserByLogin(login string) (*User, error)
	SaveUser(u User) error
}

type AuthServiceInterface interface {
	Register(req SignUpRequest) error
	Authorize(login, password string) (uuid string, err error)
	GetUser(uuid string) (*User, error)
}
