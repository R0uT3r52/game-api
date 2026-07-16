package datasource

import (
	"context"
	"fmt"
	"os"
	"game-api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found error: %s\n", e.Message)
}

func (e *DBConnectionError) Error() string {
	return fmt.Sprintf("DB connection error: %s\n", e.Message)
}

func GetDB() (*pgxpool.Pool, error) {

	// FIX:
	// Throw context through the app
	ctx := context.Background()

	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")

	connURL := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", user, password, host, port, dbname)

	db, err := pgxpool.New(ctx, connURL)

	if err != nil {
		return nil, &DBConnectionError{
			Message: err.Error(),
		}
	}

	return db, nil
}

func NewGameRepo(db *pgxpool.Pool) *GameRepository {
	return &GameRepository{
		Data: &Storage{
			Games: db,
		},
	}
}

func (r *GameRepository) Save(game *domain.Session) error {

	// FIX:
	// Throw context through the app
	ctx := context.Background()

	gameModel := FromDomain(game)

	sql := `INSERT INTO games (uuid, field, changed_at)
	VALUES ($1, $2, $3)
	ON CONFLICT (uuid) DO UPDATE
	SET field = EXCLUDED.field, changed_at = EXCLUDED.changed_at`

	_, err := r.Data.Games.Exec(ctx, sql, gameModel.UUID, gameModel.Field, gameModel.ChangedAt)
	return err
}

func (r *GameRepository) Load(uuid string) (*domain.Session, error) {

	// FIX:
	// Throw context through the app
	ctx := context.Background()

	sql := `SELECT uuid, field, changed_at FROM games WHERE uuid=$1`

	rows, _ := r.Data.Games.Query(ctx, sql, uuid)

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[GameModel])
	if err != nil {
		return nil, err
	}

	return ToDomain(&model), nil
}
