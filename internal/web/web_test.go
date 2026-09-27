package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"game-api/internal/domain"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type MockRepo struct {
	Data    sync.Map
	Latency time.Duration // DB latency imitation
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "Game not found"
}

func (m *MockRepo) Save(ctx context.Context, game *domain.Session) error {
	if m.Latency > 0 {
		time.Sleep(m.Latency)
	}
	m.Data.Store(game.UUID, *game)
	return nil
}

func (m *MockRepo) Load(ctx context.Context, uuid string) (*domain.Session, error) {
	if m.Latency > 0 {
		time.Sleep(m.Latency)
	}
	session, ok := m.Data.Load(uuid)
	if !ok {
		return nil, &domain.GameNotFoundError{UUID: uuid}
	}
	s := session.(domain.Session)
	return &s, nil
}

func (m *MockRepo) ListAvailable(ctx context.Context) ([]domain.Session, error) {
	ans := make([]domain.Session, 0)
	m.Data.Range(func(key any, value any) bool {
		s := value.(domain.Session)
		if s.Status == domain.Waiting {
			ans = append(ans, s)
		}
		return true
	})
	return ans, nil
}

func (m *MockRepo) GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]domain.Session, error) {
	ans := make([]domain.Session, 0)

	m.Data.Range(func(key, value any) bool {
		s := value.(domain.Session)
		if s.UUID == gameUUID {
			ans = append(ans, s)
		}
		return true
	})

	if len(ans) == 0 {
		return nil, &domain.GameNotFoundError{UUID: gameUUID}
	}

	clear(ans)
	ans = ans[:0:1]

	m.Data.Range(func(key any, value any) bool {
		s := value.(domain.Session)
		if (s.Player1UUID == playerUUID || s.Player2UUID == playerUUID) && (gameUUID == "" || s.UUID == gameUUID) {
			ans = append(ans, s)
		}
		return true
	})

	return ans, nil
}

func (m *MockRepo) GetUser(ctx context.Context, uuid string) (*domain.User, error) {
	return nil, nil
}

type MockAuthenticator struct{}

func (a *MockAuthenticator) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userUUID := r.Header.Get("X-Test-User")
		if userUUID == "" {
			userUUID = "test-user-uuid"
		}
		ctx := context.WithValue(r.Context(), UserUUIDKey, userUUID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func createTestServer(repo domain.GameRepositoryInterface) *httptest.Server {
	service := domain.NewGameService(repo, domain.Cross, domain.Nought)
	gameHandler := NewGameHandler(service)
	auth := &MockAuthenticator{}
	mux := http.NewServeMux()
	mux.Handle("POST /game/{uuid}", auth.Middleware(http.HandlerFunc(gameHandler.PostGame)))
	mux.Handle("POST /game/connect", auth.Middleware(http.HandlerFunc(gameHandler.ConnectGame)))
	return httptest.NewServer(mux)
}

func TestRace_ConcurrentConnect(t *testing.T) {
	ctx := context.Background()
	repo := &MockRepo{Latency: 5 * time.Millisecond}
	ts := createTestServer(repo)
	defer ts.Close()

	gameUUID := "race-connect-game"

	repo.Save(ctx, &domain.Session{
		UUID:        gameUUID,
		Status:      domain.Waiting,
		Player1UUID: "host-player",
		Player1Sign: domain.Cross,
	})

	const totalPlayers = 10
	var wg sync.WaitGroup
	wg.Add(totalPlayers)

	// channel to start goroutines at the same time
	start := make(chan struct{})

	var mu sync.Mutex
	var successCount int
	var rejectCount int

	for i := range totalPlayers {
		playerID := fmt.Sprintf("guest-player-%d", i)

		go func(player string) {
			defer wg.Done()

			body, _ := json.Marshal(ConnectGameRequest{GameUUID: gameUUID})
			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/game/connect", bytes.NewBuffer(body))
			// Simulate different users
			req.Header.Set("X-Test-User", fmt.Sprintf("guest-player-%d", i))

			<-start

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("Error on req: %v", err)
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusBadRequest {
				rejectCount++
			} else {
				t.Errorf("Unknow status code: %d", resp.StatusCode)
			}
			mu.Unlock()
		}(playerID)
	}

	// start all goroutines
	close(start)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("Race condition: successfully connected players: %d. Rejected: %d",
			successCount, rejectCount)
	}

	session, err := repo.Load(ctx, gameUUID)
	if err != nil {
		t.Fatalf("Unable to load game: %v", err)
	}
	if session.Status != domain.Turn {
		t.Errorf("Expected TurnStatus (%d), got %d", domain.Turn, session.Status)
	}
	if session.Player2UUID == "" {
		t.Errorf("Expected Player2UUID")
	}
}

func TestRace_ConcurrentMove(t *testing.T) {
	ctx := context.Background()
	repo := &MockRepo{Latency: 5 * time.Millisecond}
	ts := createTestServer(repo)
	defer ts.Close()

	gameUUID := "race-move-game"

	// Player1 turn
	repo.Save(ctx, &domain.Session{
		UUID:            gameUUID,
		Status:          domain.Turn,
		Player1UUID:     "player-1",
		Player2UUID:     "player-2",
		CurrentTurnUUID: "player-1",
		Player1Sign:     domain.Cross,
		Player2Sign:     domain.Nought,
		F:               domain.Field{},
	})

	moveA := domain.Field{Grid: [3][3]int{{domain.Cross, 0, 0}, {0, 0, 0}, {0, 0, 0}}}
	moveB := domain.Field{Grid: [3][3]int{{0, domain.Cross, 0}, {0, 0, 0}, {0, 0, 0}}}

	moves := []domain.Field{moveA, moveB}

	var wg sync.WaitGroup
	wg.Add(len(moves))

	start := make(chan struct{})

	var mu sync.Mutex
	var successCount int
	var failCount int

	for _, move := range moves {
		go func(f domain.Field) {
			defer wg.Done()

			body, _ := json.Marshal(MoveRequest{Field: f.Grid})
			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/game/"+gameUUID, bytes.NewBuffer(body))
			// Simulate player-1 making moves
			req.Header.Set("X-Test-User", "player-1")

			<-start

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("Err sending req: %v", err)
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusOK {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(move)
	}

	close(start)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("Race condition: %d turns made (expected 1). Отклонено: %d",
			successCount, failCount)
	}

	session, err := repo.Load(ctx, gameUUID)
	if err != nil {
		t.Fatalf("Error loading game: %v", err)
	}

	crossCount := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if session.F.Grid[i][j] == domain.Cross {
				crossCount++
			}
		}
	}

	if crossCount != 1 {
		t.Errorf("Expected 1 cross, got %d", crossCount)
	}

	if session.CurrentTurnUUID != "player-2" {
		t.Errorf("Must be 'player-2' turn, but got %s turn", session.CurrentTurnUUID)
	}
}

func aiVSai(ts *httptest.Server, repo domain.GameRepositoryInterface, t *testing.T, uuid string) {
	url := fmt.Sprintf("%s/game/%s", ts.URL, uuid)
	ctx := context.Background()
	// Setup game
	repo.Save(ctx, &domain.Session{
		UUID:            uuid,
		Status:          domain.Turn,
		CurrentTurnUUID: "test-user-uuid",
		Player1UUID:     "test-user-uuid",
		Player1Sign:     domain.Cross,
		IsWithBot:       true,
		Player2Sign:     domain.Nought,
	})

	for i := 0; i < 10; i++ {
		s, _ := repo.Load(ctx, uuid)
		if s.Status == domain.Win || s.Status == domain.Draw {
			return
		}

		// Player (AI for test purposes) move
		row, col := -1, -1
	findMove:
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				if s.F.Grid[r][c] == domain.Empty {
					row, col = r, c
					break findMove
				}
			}
		}
		if row == -1 {
			break
		}
		s.F.Grid[row][col] = domain.Cross

		reqBody, _ := json.Marshal(MoveRequest{Field: s.F.Grid})
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("Post failed: %v, status: %d", err, resp.StatusCode)
			return
		}

		var gameResp GameModel
		json.NewDecoder(resp.Body).Decode(&gameResp)
		resp.Body.Close()

		if gameResp.Status == domain.Win || gameResp.Status == domain.Draw {
			return
		}
	}
	t.Errorf("[UUID: %s] game not ended correctly", uuid)
}

func TestGameHandlers(t *testing.T) {
	repo := &MockRepo{}
	service := domain.NewGameService(repo, domain.Cross, domain.Nought)
	gameHandler := NewGameHandler(service)
	auth := &MockAuthenticator{}

	mux := http.NewServeMux()
	mux.Handle("POST /game/new", auth.Middleware(http.HandlerFunc(gameHandler.CreateGame)))
	mux.Handle("GET /games/available", auth.Middleware(http.HandlerFunc(gameHandler.ListGames)))
	mux.Handle("POST /game/connect", auth.Middleware(http.HandlerFunc(gameHandler.ConnectGame)))
	mux.Handle("GET /game/current", auth.Middleware(http.HandlerFunc(gameHandler.GetCurrentGame)))
	mux.Handle("GET /game/current/{uuid}", auth.Middleware(http.HandlerFunc(gameHandler.GetCurrentGame)))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. CreateGame
	createReq, _ := json.Marshal(CreateGameRequest{IsWithBot: false})
	resp, err := http.Post(ts.URL+"/game/new", "application/json", bytes.NewBuffer(createReq))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("CreateGame failed: status %v, err %v", resp.StatusCode, err)
	}
	var createResp TokenResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	resp.Body.Close()
	if createResp.UUID == "" {
		t.Fatalf("Expected non-empty game UUID")
	}

	// 2. ListGames
	resp, err = http.Get(ts.URL + "/games/available")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("ListGames failed: status %v, err %v", resp.StatusCode, err)
	}
	var games []GameModel
	json.NewDecoder(resp.Body).Decode(&games)
	resp.Body.Close()
	if len(games) != 1 || games[0].UUID != createResp.UUID {
		t.Errorf("Expected 1 available game, got %v", len(games))
	}

	// 3. ConnectGame (as a second player)
	connReq, _ := json.Marshal(ConnectGameRequest{GameUUID: createResp.UUID})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/game/connect", bytes.NewBuffer(connReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "player2-uuid")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("ConnectGame failed: status %v, err %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 4. Duplicate ConnectGame (Game already started)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/game/connect", bytes.NewBuffer(connReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "player3-uuid")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on duplicate connect, got status %v", resp.StatusCode)
	}
	resp.Body.Close()

	// 4.1 Self-connect by the creator (host joining own game)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/game/connect", bytes.NewBuffer(connReq))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req) // no X-Test-User -> creator = test-user-uuid
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on self-connect, got status %v", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. GetCurrentGame
	// req, _ := http.NewRequest(http.MethodGet, ts.URL+"/game/current", bytes.NewBuffer(connReq))
	resp, err = http.Get(ts.URL + "/game/current/" + createResp.UUID)
	// resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GetCurrentGame failed: status %v, err %v", resp.StatusCode, err)
	}
	var currentGames []GameModel
	json.NewDecoder(resp.Body).Decode(&currentGames)
	resp.Body.Close()
	if len(currentGames) != 1 {
		t.Errorf("Expected 1 active game, got %v", len(currentGames))
	}
}

func TestGameHandlerErrorBranches(t *testing.T) {
	ctx := context.Background()
	repo := &MockRepo{}
	service := domain.NewGameService(repo, domain.Cross, domain.Nought)
	gameHandler := NewGameHandler(service)
	auth := &MockAuthenticator{}

	mux := http.NewServeMux()
	mux.Handle("POST /game/new", auth.Middleware(http.HandlerFunc(gameHandler.CreateGame)))
	mux.Handle("GET /games/available", auth.Middleware(http.HandlerFunc(gameHandler.ListGames)))
	mux.Handle("POST /game/connect", auth.Middleware(http.HandlerFunc(gameHandler.ConnectGame)))
	mux.Handle("GET /game/current", auth.Middleware(http.HandlerFunc(gameHandler.GetCurrentGame)))
	mux.Handle("GET /game/current/{uuid}", auth.Middleware(http.HandlerFunc(gameHandler.GetCurrentGame)))
	mux.Handle("POST /game/{uuid}", auth.Middleware(http.HandlerFunc(gameHandler.PostGame)))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Invalid JSON body
	badJSON := bytes.NewBuffer([]byte(`{invalid-json`))
	resp, _ := http.Post(ts.URL+"/game/new", "application/json", badJSON)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on invalid JSON in CreateGame, got %d", resp.StatusCode)
	}

	badJSON = bytes.NewBuffer([]byte(`{invalid-json`))
	resp, _ = http.Post(ts.URL+"/game/connect", "application/json", badJSON)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on invalid JSON in ConnectGame, got %d", resp.StatusCode)
	}

	badJSON = bytes.NewBuffer([]byte(`{invalid-json`))
	// req, _ := http.NewRequest(http.MethodGet, ts.URL+"/game/current", badJSON)

	resp, _ = http.Get(ts.URL + "/game/current/123123r53ot")
	// resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found on invalid path value in GetCurrentGame, got %d", resp.StatusCode)
	}

	badJSON = bytes.NewBuffer([]byte(`{invalid-json`))
	resp, _ = http.Post(ts.URL+"/game/nonexistent-id", "application/json", badJSON)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on invalid JSON in PostGame, got %d", resp.StatusCode)
	}

	// PostGame GameNotFoundError (404)
	validMove, _ := json.Marshal(MoveRequest{Field: [3][3]int{{domain.Cross, 0, 0}, {0, 0, 0}, {0, 0, 0}}})
	resp, _ = http.Post(ts.URL+"/game/nonexistent-id", "application/json", bytes.NewBuffer(validMove))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found on nonexistent game PostGame, got %d", resp.StatusCode)
	}

	// PostGame ValidationError (400)
	gID, _ := service.CreateGame(ctx, "test-user-uuid", false)
	_ = service.Connect(ctx, "p2", gID)
	// Invalid move (changing 2 cells)
	invalidMove, _ := json.Marshal(MoveRequest{Field: [3][3]int{{domain.Cross, domain.Cross, 0}, {0, 0, 0}, {0, 0, 0}}})
	resp, _ = http.Post(ts.URL+"/game/"+gID, "application/json", bytes.NewBuffer(invalidMove))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on invalid move, got %d", resp.StatusCode)
	}
}

type MockErrGameService struct{}

func (m *MockErrGameService) MakeAiMove(ctx context.Context, uuid string) (*domain.Session, error) {
	return nil, errors.New("service err")
}
func (m *MockErrGameService) MakeMove(ctx context.Context, gameUUID, playerUUID string, newField domain.Field) (*domain.Session, error) {
	return nil, errors.New("service err")
}
func (m *MockErrGameService) ValidateField(ctx context.Context, uuid string, newField domain.Field) error {
	return errors.New("service err")
}
func (m *MockErrGameService) CheckGameEnd(ctx context.Context, uuid string) (bool, int, error) {
	return false, 0, errors.New("service err")
}
func (m *MockErrGameService) CreateGame(ctx context.Context, p1 string, withBot bool) (string, error) {
	return "", errors.New("service err")
}
func (m *MockErrGameService) GetAvailableGames(ctx context.Context) ([]domain.Session, error) {
	return nil, errors.New("service err")
}
func (m *MockErrGameService) GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]domain.Session, error) {
	return nil, errors.New("service err")
}
func (m *MockErrGameService) Connect(ctx context.Context, p2UUID, gameUUID string) error {
	return errors.New("service err")
}

func TestGameHandlerServiceErrors(t *testing.T) {
	errSvc := &MockErrGameService{}
	handler := NewGameHandler(errSvc)
	auth := &MockAuthenticator{}

	mux := http.NewServeMux()
	mux.Handle("POST /game/new", auth.Middleware(http.HandlerFunc(handler.CreateGame)))
	mux.Handle("GET /games/available", auth.Middleware(http.HandlerFunc(handler.ListGames)))
	mux.Handle("POST /game/connect", auth.Middleware(http.HandlerFunc(handler.ConnectGame)))
	mux.Handle("GET /game/current", auth.Middleware(http.HandlerFunc(handler.GetCurrentGame)))
	mux.Handle("GET /game/current/{uuid}", auth.Middleware(http.HandlerFunc(handler.GetCurrentGame)))
	mux.Handle("POST /game/{uuid}", auth.Middleware(http.HandlerFunc(handler.PostGame)))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(CreateGameRequest{})
	resp, _ := http.Post(ts.URL+"/game/new", "application/json", bytes.NewBuffer(body))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 for CreateGame error, got %d", resp.StatusCode)
	}

	resp, _ = http.Get(ts.URL + "/games/available")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 for ListGames error, got %d", resp.StatusCode)
	}

	body, _ = json.Marshal(ConnectGameRequest{})
	resp, _ = http.Post(ts.URL+"/game/connect", "application/json", bytes.NewBuffer(body))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 for ConnectGame error, got %d", resp.StatusCode)
	}

	// req, _ := http.NewRequest(http.MethodGet, ts.URL+"/game/current"+"/", bytes.NewBuffer(body))
	resp, _ = http.Get(ts.URL + "/game/current")
	// resp, _ = http.DefaultClient.Do(req)
	// t.Logf("---------------- Resp status code: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusBadRequest { // err != nil in GetCurrentGames returns 400
		t.Errorf("Expected 400 for GetCurrentGame error, got %d", resp.StatusCode)
	}

	body, _ = json.Marshal(MoveRequest{})
	resp, _ = http.Post(ts.URL+"/game/some-id", "application/json", bytes.NewBuffer(body))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 for PostGame error, got %d", resp.StatusCode)
	}
}
