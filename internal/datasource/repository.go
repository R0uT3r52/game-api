package datasource

import (
	"context"
	"errors"
	"fmt"
	"game-api/internal/domain"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetDB(ctx context.Context) (*pgxpool.Pool, error) {

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

func (r *GameRepository) GetLeaderboard(ctx context.Context, limit int64) ([]domain.LeaderBoard, error) {

	sqlAll := `
		WITH user_games AS (
			SELECT u.uuid AS player_uuid, u.login AS player_login, g.winner_uuid, g.status
			FROM users u
			JOIN games g ON u.uuid=g.player1_uuid OR u.uuid=g.player2_uuid
			WHERE g.status=$1 OR g.status=$2
		)

		SELECT player_uuid, player_login,
		ROUND(
			COUNT(CASE WHEN winner_uuid = player_uuid THEN 1 END)::NUMERIC /
			COALESCE(NULLIF(COUNT(CASE WHEN winner_uuid != player_uuid OR winner_uuid IS NULL THEN 1 END), 0), 1),
			2
		)::FLOAT AS winrate
		FROM user_games
		GROUP BY player_uuid, player_login
		ORDER BY winrate DESC NULLS LAST
		LIMIT $3;
	`

	rows, err := r.Data.Games.Query(ctx, sqlAll, domain.Win, domain.Draw, limit)
	if err != nil {
		return nil, err
	}

	data, err := pgx.CollectRows(rows, pgx.RowToStructByName[LeaderBoardModel])
	if err != nil {
		return nil, err
	}

	ans := make([]domain.LeaderBoard, 0, len(data))
	for i := range data {
		item := LeaderboardToDomain(&data[i])
		ans = append(ans, *item)
	}

	return ans, nil
}

func (r *GameRepository) ListAvailable(ctx context.Context) ([]domain.Session, error) {
	// ctx := context.Background()
	ans := make([]domain.Session, 0)
	sql := `SELECT * FROM games WHERE status=$1 AND is_with_bot is FALSE;`

	rows, err := r.Data.Games.Query(ctx, sql, domain.Waiting)
	if err != nil {
		return nil, err
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[GameModel])
	if err != nil {
		return nil, err
	}

	for _, elem := range models {
		s := ToDomain(&elem)
		ans = append(ans, *s)
	}

	return ans, nil
}

func (r *GameRepository) GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]domain.Session, error) {
	if gameUUID == "" {
		// ctx := context.Background()
		ans := make([]domain.Session, 0)
		sql := `SELECT * FROM games WHERE player1_uuid=$1 OR player2_uuid=$1;`
		rows, err := r.Data.Games.Query(ctx, sql, playerUUID)
		if err != nil {
			return nil, err
		}
		models, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[GameModel])
		if err != nil {
			return nil, err
		}
		for _, elem := range models {
			ans = append(ans, *ToDomain(elem))
		}
		return ans, nil
	}

	session, err := r.Load(ctx, gameUUID)
	if err != nil {
		return nil, err
	}

	if session.Player1UUID != playerUUID && session.Player2UUID != playerUUID {
		return []domain.Session{}, nil
	}

	return []domain.Session{*session}, nil
}

func (r *GameRepository) GetFinishedGames(ctx context.Context, playerUUID string) ([]domain.Session, error) {

	sql := `SELECT * FROM games WHERE (player1_uuid=$1 OR player2_uuid=$1) AND (status=$2 OR status=$3) ORDER BY created_at DESC;`

	rows, err := r.Data.Games.Query(ctx, sql, playerUUID, domain.Draw, domain.Win)
	if err != nil {
		return nil, err
	}

	data, err := pgx.CollectRows(rows, pgx.RowToStructByName[GameModel])
	if err != nil {
		return nil, err
	}

	ans := make([]domain.Session, 0, len(data))
	for _, elem := range data {
		ans = append(ans, *ToDomain(&elem))
	}

	return ans, nil
}

func (r *GameRepository) Save(ctx context.Context, game *domain.Session) error {

	gameModel := FromDomain(game)

	sql := `INSERT INTO games (uuid, field, player1_uuid, player2_uuid, current_turn_uuid, status, is_with_bot, winner_uuid, player1_sign, player2_sign, changed_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (uuid) DO UPDATE
	SET field = EXCLUDED.field,
		player1_uuid = EXCLUDED.player1_uuid,
		player2_uuid = EXCLUDED.player2_uuid,
		current_turn_uuid = EXCLUDED.current_turn_uuid,
		status = EXCLUDED.status,
		is_with_bot = EXCLUDED.is_with_bot,
		winner_uuid = EXCLUDED.winner_uuid,
		player1_sign = EXCLUDED.player1_sign,
		player2_sign = EXCLUDED.player2_sign,
		changed_at = EXCLUDED.changed_at`

	_, err := r.Data.Games.Exec(ctx, sql,
		gameModel.UUID,
		gameModel.Field,
		gameModel.Player1UUID,
		gameModel.Player2UUID,
		gameModel.CurrentTurnUUID,
		gameModel.Status,
		gameModel.IsWithBot,
		gameModel.WinnerUUID,
		gameModel.Player1Sign,
		gameModel.Player2Sign,
		gameModel.ChangedAt)
	return err
}

func (r *GameRepository) Load(ctx context.Context, uuid string) (*domain.Session, error) {

	sql := `SELECT * FROM games WHERE uuid=$1`

	rows, err := r.Data.Games.Query(ctx, sql, uuid)
	if err != nil {
		return nil, err
	}

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[GameModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.GameNotFoundError{UUID: uuid}
		}
		return nil, err
	}

	return ToDomain(&model), nil
}

func (ur *UserRepository) GetUser(ctx context.Context, uuid string) (*domain.User, error) {
	// db user => domain User

	// ctx := context.Background()
	sql := `SELECT uuid, login, password_hash, created_at FROM users WHERE uuid=$1`

	rows, err := ur.Data.Query(ctx, sql, uuid)
	if err != nil {
		return nil, err
	}

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		return nil, err
	}

	return UserToDomain(&model), nil
}

func (ur *UserRepository) GetUserByLogin(ctx context.Context, login string) (*domain.User, error) {
	// ctx := context.Background()

	sql := `SELECT uuid, login, password_hash, created_at FROM users WHERE login=$1`

	rows, err := ur.Data.Query(ctx, sql, login)
	if err != nil {
		return nil, err
	}

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[UserModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return UserToDomain(&model), nil
}

func (ur *UserRepository) SaveUser(ctx context.Context, u domain.User) error {
	// domain User => db user

	// ctx := context.Background()

	sql := `INSERT INTO users (uuid, login, password_hash, created_at)
			VALUES ($1, $2, $3, $4)`

	model := UserFromDomain(&u)

	_, err := ur.Data.Exec(ctx, sql, model.UUID, model.Login, model.PasswordHash, model.CreatedAt)
	var pgErr *pgconn.PgError
	// Check for login not unique
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return &domain.UserAlreadyExistsError{Login: u.Login}
	}
	return err
}
