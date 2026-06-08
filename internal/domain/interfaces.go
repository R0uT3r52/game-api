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
