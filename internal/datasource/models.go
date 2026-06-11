package datasource

import (
	"sync"
	"time"
)

type NotFoundError struct {
	Message string
}

type Storage struct {
	Games sync.Map
}

type GameRepository struct {
	Data *Storage
}

type GameModel struct {
	UUID      string
	Field     [3][3]int
	ChangedAt time.Time
}
