package datasource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"game-api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (ur *UserRepository) GetUser(uuid string) (*domain.User, error) {
	// db user => domain User

	ctx := context.Background()
	sql := `SELECT uuid, login, password_hash, created_at FROM users WHERE uuid=$1`

	rows, _ := ur.Data.Query(ctx, sql, uuid)

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		return nil, err
	}

	return UserToDomain(&model), nil
}

func (ur *UserRepository) GetUserByLogin(login string) (*domain.User, error) {
	ctx := context.Background()

	sql := `SELECT uuid, login, password_hash, created_at FROM users WHERE login=$1`

	rows, _ := ur.Data.Query(ctx, sql, login)

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return UserToDomain(&model), nil
}

func (ur *UserRepository) SaveUser(u domain.User) error {
	// domain User => db user

	ctx := context.Background()

	sql := `INSERT INTO users (uuid, login, password_hash, created_at)
			VALUES ($1, $2, $3, $4)`

	model := UserFromDomain(&u)

	_, err := ur.Data.Exec(ctx, sql, model.UUID, model.Login, model.PasswordHash, model.CreatedAt)
	return err
}
