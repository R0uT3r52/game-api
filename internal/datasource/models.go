package datasource

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type NotFoundError struct {
	Message string
}

type DBConnectionError struct {
	Message string
}

type Storage struct {
	Games *pgxpool.Pool
}

type GameRepository struct {
	Data *Storage
}

type GameModel struct {
	UUID      string    `db:"uuid"`
	Field     [3][3]int `db:"field"`
	ChangedAt time.Time `db:"changed_at"`
}
