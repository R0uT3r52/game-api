package datasource

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	Games *pgxpool.Pool
}

type GameRepository struct {
	Data *Storage
}

type GameModel struct {
	UUID            string    `db:"uuid"`
	Field           [3][3]int `db:"field"`
	Player1UUID     *string   `db:"player1_uuid"`
	Player2UUID     *string   `db:"player2_uuid"`
	CurrentTurnUUID *string   `db:"current_turn_uuid"`
	Status          int       `db:"status"`
	IsWithBot       bool      `db:"is_with_bot"`
	WinnerUUID      *string   `db:"winner_uuid"`
	Player1Sign     int       `db:"player1_sign"`
	Player2Sign     int       `db:"player2_sign"`
	ChangedAt       time.Time `db:"changed_at"`
}

// GetUser(uuid string) (*User, error)
// SaveUser() error
type UserModel struct {
	UUID         string    `db:"uuid"`
	Login        string    `db:"login"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

type UserRepository struct {
	Data *pgxpool.Pool
}
