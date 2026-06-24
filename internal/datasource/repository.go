package datasource

import (
	"fmt"
	"game-api/internal/domain"
	"sync"
)

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found error: %s\n", e.Message)
}

func NewGameRepo() *GameRepository {
	return &GameRepository{
		Data: &Storage{
			Games: sync.Map{},
		},
	}
}

func (r *GameRepository) Save(game *domain.Session) error {
	gameModel := FromDomain(game)

	r.Data.Games.Store(game.UUID, &gameModel)
	return nil
}

func (r *GameRepository) Load(uuid string) (*domain.Session, error) {
	val, ok := r.Data.Games.Load(uuid)
	if !ok {
		return nil, &NotFoundError{Message: "game not found with this session id"}
	}

	gameModel := ToDomain(val.(*GameModel))

	return gameModel, nil
}
