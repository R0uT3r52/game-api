package domain

type GameRepositoryInterface interface {
	Save(game *Session) error
	Load(uuid string) (*Session, error)
}

type GameServiceInterface interface {
	MakeAiMove(uuid string) (*Session, error)
	ValidateField(uuid string, newField Field) error
	CheckGameEnd(uuid string) (isEnded bool, winner int, err error)
}

type UserServiceInterface interface {
	GetUser(uuid string) (*User, error)
	GetUserByLogin(login string) (*User, error)
	SaveUser(u User) error
}

type AuthServiceInterface interface {
	Register(req SignUpRequest) error
	Authorize(authHeader string) (uuid string, err error)
}
