package datasource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"game-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	pgOnce      sync.Once
	pgContainer *postgres.PostgresContainer
	pgPool      *pgxpool.Pool
	pgInitErr   error
)

func TestMain(m *testing.M) {
	code := m.Run()

	if pgPool != nil {
		pgPool.Close()
	}
	if pgContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = pgContainer.Terminate(ctx)
		cancel()
	}

	os.Exit(code)
}

func requireTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	pgOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		initScript := filepath.Join("..", "..", "migrations", "database.sql")
		c, err := postgres.Run(
			ctx,
			"postgres:16.3-alpine",
			postgres.WithInitScripts(initScript),
			postgres.WithDatabase("testdb"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("postgres"),
			postgres.BasicWaitStrategies(),
		)
		if err != nil {
			pgInitErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		pgContainer = c

		connStr, err := c.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			pgInitErr = fmt.Errorf("get connection string: %w", err)
			return
		}

		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			pgInitErr = fmt.Errorf("create pgxpool: %w", err)
			return
		}

		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			pgInitErr = fmt.Errorf("ping postgres: %w", err)
			return
		}
		pgPool = pool
	})

	if pgInitErr != nil {
		t.Fatalf("postgres test setup failed: %v", pgInitErr)
	}
	return pgPool
}

func resetDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "TRUNCATE TABLE games, users;"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func createTestUser(t *testing.T, repo *UserRepository, login string) domain.User {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := domain.User{
		UUID:         uuid.NewString(),
		Login:        login,
		PasswordHash: "hashed-password",
	}
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser(%s): %v", login, err)
	}
	return u
}

func TestUserRepositoryIntegration(t *testing.T) {
	pool := requireTestPool(t)
	resetDB(t, pool)

	repo := &UserRepository{Data: pool}
	ctx := context.Background()

	u := domain.User{
		UUID:         uuid.NewString(),
		Login:        "alice",
		PasswordHash: "hash-123",
	}

	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser failed: %v", err)
	}

	gotByID, err := repo.GetUser(ctx, u.UUID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if gotByID.UUID != u.UUID || gotByID.Login != u.Login || gotByID.PasswordHash != u.PasswordHash {
		t.Fatalf("GetUser mismatch: got %+v, want %+v", gotByID, u)
	}

	gotByLogin, err := repo.GetUserByLogin(ctx, u.Login)
	if err != nil {
		t.Fatalf("GetUserByLogin failed: %v", err)
	}
	if gotByLogin == nil || gotByLogin.UUID != u.UUID {
		t.Fatalf("GetUserByLogin mismatch: got %+v", gotByLogin)
	}

	missing, err := repo.GetUserByLogin(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("GetUserByLogin(missing) unexpected error: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing login, got %+v", missing)
	}

	if _, err := repo.GetUser(ctx, uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows for missing user UUID, got %v", err)
	}

	dup := domain.User{
		UUID:         uuid.NewString(),
		Login:        "alice",
		PasswordHash: "other-hash",
	}
	err = repo.SaveUser(ctx, dup)
	var existsErr *domain.UserAlreadyExistsError
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected *domain.UserAlreadyExistsError, got %v", err)
	}
}

func TestGameRepositoryCRUDAndQueriesIntegration(t *testing.T) {
	pool := requireTestPool(t)
	resetDB(t, pool)

	userRepo := &UserRepository{Data: pool}
	gameRepo := NewGameRepo(pool)
	ctx := context.Background()

	p1 := createTestUser(t, userRepo, "player1")
	p2 := createTestUser(t, userRepo, "player2")
	p3 := createTestUser(t, userRepo, "player3")

	// Save & Load (insert + upsert)
	waitingID := uuid.NewString()
	waitingGame := &domain.Session{
		UUID:            waitingID,
		F:               domain.Field{},
		Player1UUID:     p1.UUID,
		CurrentTurnUUID: p1.UUID,
		Status:          domain.Waiting,
		IsWithBot:       false,
		Player1Sign:     domain.Cross,
		Player2Sign:     domain.Empty,
	}
	if err := gameRepo.Save(ctx, waitingGame); err != nil {
		t.Fatalf("Save waiting game: %v", err)
	}

	loaded, err := gameRepo.Load(ctx, waitingID)
	if err != nil {
		t.Fatalf("Load waiting game: %v", err)
	}
	if loaded.UUID != waitingID || loaded.Player1UUID != p1.UUID || loaded.Player2UUID != "" || loaded.Status != domain.Waiting {
		t.Fatalf("loaded game mismatch: %+v", loaded)
	}

	// Upsert same UUID
	waitingGame.F.Grid[1][1] = domain.Cross
	if err := gameRepo.Save(ctx, waitingGame); err != nil {
		t.Fatalf("Save upsert: %v", err)
	}
	loaded, err = gameRepo.Load(ctx, waitingID)
	if err != nil {
		t.Fatalf("Load after upsert: %v", err)
	}
	if loaded.F.Grid[1][1] != domain.Cross {
		t.Fatalf("expected Grid[1][1]=%d after upsert, got %d", domain.Cross, loaded.F.Grid[1][1])
	}

	// Missing game returns *domain.GameNotFoundError
	_, err = gameRepo.Load(ctx, uuid.NewString())
	var notFound *domain.GameNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *domain.GameNotFoundError, got %v", err)
	}

	// Bot game (Waiting but IsWithBot=true should not appear in ListAvailable)
	botWaitingID := uuid.NewString()
	if err := gameRepo.Save(ctx, &domain.Session{
		UUID:            botWaitingID,
		Player1UUID:     p1.UUID,
		CurrentTurnUUID: p1.UUID,
		Status:          domain.Waiting,
		IsWithBot:       true,
		Player1Sign:     domain.Cross,
		Player2Sign:     domain.Nought,
	}); err != nil {
		t.Fatalf("Save bot game: %v", err)
	}

	avail, err := gameRepo.ListAvailable(ctx)
	if err != nil {
		t.Fatalf("ListAvailable: %v", err)
	}
	if len(avail) != 1 || avail[0].UUID != waitingID {
		t.Fatalf("expected only waiting PvP game %s, got %+v", waitingID, avail)
	}

	// Finished games for leaderboard and GetFinishedGames
	fin1 := &domain.Session{
		UUID:        uuid.NewString(),
		Player1UUID: p1.UUID,
		Player2UUID: p2.UUID,
		Status:      domain.Win,
		WinnerUUID:  p1.UUID,
		Player1Sign: domain.Cross,
		Player2Sign: domain.Nought,
	}
	fin2 := &domain.Session{
		UUID:        uuid.NewString(),
		Player1UUID: p2.UUID,
		Player2UUID: p3.UUID,
		Status:      domain.Win,
		WinnerUUID:  p2.UUID,
		Player1Sign: domain.Cross,
		Player2Sign: domain.Nought,
	}
	finDraw := &domain.Session{
		UUID:        uuid.NewString(),
		Player1UUID: p1.UUID,
		Player2UUID: p3.UUID,
		Status:      domain.Draw,
		Player1Sign: domain.Cross,
		Player2Sign: domain.Nought,
	}
	for _, g := range []*domain.Session{fin1, fin2, finDraw} {
		if err := gameRepo.Save(ctx, g); err != nil {
			t.Fatalf("Save finished game %s: %v", g.UUID, err)
		}
		// Ensure distinct created_at ordering for GetFinishedGames
		time.Sleep(15 * time.Millisecond)
	}

	// GetCurrentGames
	p1Games, err := gameRepo.GetCurrentGames(ctx, "", p1.UUID)
	if err != nil {
		t.Fatalf("GetCurrentGames all: %v", err)
	}
	if len(p1Games) != 4 {
		t.Fatalf("expected 4 games for p1, got %d", len(p1Games))
	}

	specific, err := gameRepo.GetCurrentGames(ctx, fin1.UUID, p1.UUID)
	if err != nil {
		t.Fatalf("GetCurrentGames specific: %v", err)
	}
	if len(specific) != 1 || specific[0].UUID != fin1.UUID {
		t.Fatalf("expected fin1 game, got %+v", specific)
	}

	unrelated, err := gameRepo.GetCurrentGames(ctx, fin1.UUID, p3.UUID)
	if err != nil {
		t.Fatalf("GetCurrentGames unrelated: %v", err)
	}
	if len(unrelated) != 0 {
		t.Fatalf("expected empty slice for non-participant, got %+v", unrelated)
	}

	// GetFinishedGames ordered DESC by created_at
	finishedP1, err := gameRepo.GetFinishedGames(ctx, p1.UUID)
	if err != nil {
		t.Fatalf("GetFinishedGames: %v", err)
	}
	if len(finishedP1) != 2 {
		t.Fatalf("expected 2 finished games for p1, got %d", len(finishedP1))
	}
	if finishedP1[0].UUID != finDraw.UUID || finishedP1[1].UUID != fin1.UUID {
		t.Fatalf("expected finished games ordered DESC [%s, %s], got [%s, %s]",
			finDraw.UUID, fin1.UUID, finishedP1[0].UUID, finishedP1[1].UUID)
	}

	// GetLeaderboard: p1 (1 win, 1 draw -> 1.0), p2 (1 win, 1 loss -> 1.0), p3 (0 wins, 2 non-wins -> 0.0)
	lb, err := gameRepo.GetLeaderboard(ctx, 2)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(lb) != 2 {
		t.Fatalf("expected limit 2 leaderboard rows, got %d", len(lb))
	}
	if lb[0].WinRate < lb[1].WinRate {
		t.Fatalf("expected descending winrate order, got %+v", lb)
	}
	lbAll, err := gameRepo.GetLeaderboard(ctx, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard all: %v", err)
	}
	if len(lbAll) != 3 || lbAll[2].PlayerUUID != p3.UUID || lbAll[2].WinRate != 0 {
		t.Fatalf("expected p3 last with winrate 0, got %+v", lbAll)
	}
}

func TestConnectPlayerIntegration(t *testing.T) {
	pool := requireTestPool(t)
	resetDB(t, pool)

	userRepo := &UserRepository{Data: pool}
	gameRepo := NewGameRepo(pool)
	ctx := context.Background()

	p1 := createTestUser(t, userRepo, "host")
	p2 := createTestUser(t, userRepo, "guest")

	// Nonexistent game -> *domain.GameNotFoundError
	err := gameRepo.ConnectPlayer(ctx, uuid.NewString(), p2.UUID)
	var notFound *domain.GameNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *domain.GameNotFoundError, got %v", err)
	}

	gameID := uuid.NewString()
	if err := gameRepo.Save(ctx, &domain.Session{
		UUID:            gameID,
		Player1UUID:     p1.UUID,
		CurrentTurnUUID: p1.UUID,
		Status:          domain.Waiting,
		Player1Sign:     domain.Cross,
		Player2Sign:     domain.Empty,
	}); err != nil {
		t.Fatalf("Save game: %v", err)
	}

	// Host connecting to own game -> ErrUserAlreadyInGame
	if err := gameRepo.ConnectPlayer(ctx, gameID, p1.UUID); !errors.Is(err, domain.ErrUserAlreadyInGame) {
		t.Fatalf("expected ErrUserAlreadyInGame, got %v", err)
	}

	// Concurrent race: 10 distinct players try to join simultaneously
	const contenders = 10
	challengers := make([]domain.User, contenders)
	for i := range challengers {
		challengers[i] = createTestUser(t, userRepo, fmt.Sprintf("challenger-%d", i))
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var startedErrCount atomic.Int32
	errs := make(chan error, contenders)

	for i := range contenders {
		wg.Add(1)
		go func(playerID string) {
			defer wg.Done()
			<-start
			err := gameRepo.ConnectPlayer(ctx, gameID, playerID)
			if err == nil {
				successCount.Add(1)
			} else if errors.Is(err, domain.ErrGameAlreadyStarted) {
				startedErrCount.Add(1)
			} else {
				errs <- err
			}
		}(challengers[i].UUID)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("unexpected error in concurrent ConnectPlayer: %v", err)
	}

	if successCount.Load() != 1 {
		t.Fatalf("expected exactly 1 successful ConnectPlayer, got %d", successCount.Load())
	}
	if startedErrCount.Load() != contenders-1 {
		t.Fatalf("expected %d ErrGameAlreadyStarted, got %d", contenders-1, startedErrCount.Load())
	}

	updated, err := gameRepo.Load(ctx, gameID)
	if err != nil {
		t.Fatalf("Load after ConnectPlayer: %v", err)
	}
	if updated.Status != domain.Turn || updated.Player2UUID == "" || updated.Player2Sign != domain.Nought {
		t.Fatalf("unexpected game state after ConnectPlayer: %+v", updated)
	}
}

func TestWithTxAndLoadForUpdateIntegration(t *testing.T) {
	pool := requireTestPool(t)
	resetDB(t, pool)

	userRepo := &UserRepository{Data: pool}
	gameRepo := NewGameRepo(pool)
	ctx := context.Background()

	p1 := createTestUser(t, userRepo, "tx-p1")
	p2 := createTestUser(t, userRepo, "tx-p2")

	gameID := uuid.NewString()
	if err := gameRepo.Save(ctx, &domain.Session{
		UUID:            gameID,
		Player1UUID:     p1.UUID,
		Player2UUID:     p2.UUID,
		CurrentTurnUUID: p1.UUID,
		Status:          domain.Turn,
		Player1Sign:     domain.Cross,
		Player2Sign:     domain.Nought,
	}); err != nil {
		t.Fatalf("Save initial game: %v", err)
	}

	// 1. Rollback on error
	sentinel := errors.New("rollback trigger")
	err := gameRepo.WithTx(ctx, func(txRepo domain.GameRepositoryInterface) error {
		s, err := txRepo.LoadForUpdate(ctx, gameID)
		if err != nil {
			return err
		}
		s.F.Grid[2][2] = domain.Cross
		if err := txRepo.Save(ctx, s); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	afterRollback, err := gameRepo.Load(ctx, gameID)
	if err != nil {
		t.Fatalf("Load after rollback: %v", err)
	}
	if afterRollback.F.Grid[2][2] != domain.Empty {
		t.Fatalf("expected rollback to discard Grid[2][2], got %d", afterRollback.F.Grid[2][2])
	}

	// 2. Row lock (FOR UPDATE) serializes concurrent transactions without lost updates
	tx1Locked := make(chan struct{})
	tx2Done := make(chan error, 1)

	err = gameRepo.WithTx(ctx, func(txRepo1 domain.GameRepositoryInterface) error {
		s1, err := txRepo1.LoadForUpdate(ctx, gameID)
		if err != nil {
			close(tx1Locked)
			return err
		}
		close(tx1Locked)

		// Start tx2 after tx1 holds the row lock; tx2 must block on LoadForUpdate until tx1 commits.
		go func() {
			tx2Done <- gameRepo.WithTx(ctx, func(txRepo2 domain.GameRepositoryInterface) error {
				s2, err := txRepo2.LoadForUpdate(ctx, gameID)
				if err != nil {
					return err
				}
				if s2.F.Grid[0][0] != domain.Cross {
					return fmt.Errorf("tx2 read stale Grid[0][0]=%d, expected %d", s2.F.Grid[0][0], domain.Cross)
				}
				s2.F.Grid[0][1] = domain.Nought
				return txRepo2.Save(ctx, s2)
			})
		}()

		time.Sleep(150 * time.Millisecond)
		s1.F.Grid[0][0] = domain.Cross
		return txRepo1.Save(ctx, s1)
	})
	if err != nil {
		t.Fatalf("tx1 failed: %v", err)
	}

	<-tx1Locked
	if err := <-tx2Done; err != nil {
		t.Fatalf("tx2 failed: %v", err)
	}

	finalState, err := gameRepo.Load(ctx, gameID)
	if err != nil {
		t.Fatalf("Load final state: %v", err)
	}
	if finalState.F.Grid[0][0] != domain.Cross || finalState.F.Grid[0][1] != domain.Nought {
		t.Fatalf("lost update detected in Grid: %+v", finalState.F.Grid)
	}
}

func TestGameServiceWithPostgresConcurrencyIntegration(t *testing.T) {
	pool := requireTestPool(t)
	resetDB(t, pool)

	userRepo := &UserRepository{Data: pool}
	gameRepo := NewGameRepo(pool)
	svc := domain.NewGameService(gameRepo, domain.Cross, domain.Nought)
	ctx := context.Background()

	p1 := createTestUser(t, userRepo, "svc-p1")
	p2 := createTestUser(t, userRepo, "svc-p2")
	p3 := createTestUser(t, userRepo, "svc-p3")

	gameID, err := svc.CreateGame(ctx, p1.UUID, false)
	if err != nil {
		t.Fatalf("CreateGame PvP: %v", err)
	}

	// Concurrent Connect race through GameService
	startConnect := make(chan struct{})
	var connectWG sync.WaitGroup
	var connectOK atomic.Int32
	connectErrs := make([]error, 2)

	for idx, pid := range []string{p2.UUID, p3.UUID} {
		connectWG.Add(1)
		go func(i int, playerID string) {
			defer connectWG.Done()
			<-startConnect
			err := svc.Connect(ctx, playerID, gameID)
			if err == nil {
				connectOK.Add(1)
			}
			connectErrs[i] = err
		}(idx, pid)
	}
	close(startConnect)
	connectWG.Wait()

	if connectOK.Load() != 1 {
		t.Fatalf("expected 1 Connect to succeed, got %d (errs: %v)", connectOK.Load(), connectErrs)
	}

	// Concurrent double-move attempt by p1 on the same turn
	moveA := domain.Field{Grid: [3][3]int{{domain.Cross, 0, 0}, {0, 0, 0}, {0, 0, 0}}}
	moveB := domain.Field{Grid: [3][3]int{{0, domain.Cross, 0}, {0, 0, 0}, {0, 0, 0}}}

	startMove := make(chan struct{})
	var moveWG sync.WaitGroup
	var moveOK atomic.Int32
	var moveFailed atomic.Int32

	for _, f := range []domain.Field{moveA, moveB} {
		moveWG.Add(1)
		go func(field domain.Field) {
			defer moveWG.Done()
			<-startMove
			_, err := svc.MakeMove(ctx, gameID, p1.UUID, field)
			if err == nil {
				moveOK.Add(1)
			} else {
				moveFailed.Add(1)
			}
		}(f)
	}
	close(startMove)
	moveWG.Wait()

	if moveOK.Load() != 1 || moveFailed.Load() != 1 {
		t.Fatalf("expected 1 move success and 1 failure, got ok=%d fail=%d", moveOK.Load(), moveFailed.Load())
	}

	afterMove, err := gameRepo.Load(ctx, gameID)
	if err != nil {
		t.Fatalf("Load after concurrent MakeMove: %v", err)
	}
	if afterMove.CurrentTurnUUID == p1.UUID {
		t.Fatalf("expected turn to switch away from p1, still %s", afterMove.CurrentTurnUUID)
	}

	// Bot game end-to-end persistence
	botGameID, err := svc.CreateGame(ctx, p1.UUID, true)
	if err != nil {
		t.Fatalf("CreateGame with bot: %v", err)
	}
	res, err := svc.MakeMove(ctx, botGameID, p1.UUID, moveA)
	if err != nil {
		t.Fatalf("MakeMove against bot: %v", err)
	}

	nonEmpty := 0
	for i := range res.F.Grid {
		for j := range res.F.Grid[i] {
			if res.F.Grid[i][j] != domain.Empty {
				nonEmpty++
			}
		}
	}
	if nonEmpty != 2 {
		t.Fatalf("expected 2 cells filled (player + bot), got %d in %+v", nonEmpty, res.F.Grid)
	}

	persistedBot, err := gameRepo.Load(ctx, botGameID)
	if err != nil {
		t.Fatalf("Load bot game: %v", err)
	}
	if persistedBot.F.Grid != res.F.Grid {
		t.Fatalf("persisted bot grid mismatch: got %+v, want %+v", persistedBot.F.Grid, res.F.Grid)
	}
}
