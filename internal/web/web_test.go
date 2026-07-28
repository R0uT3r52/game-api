package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"game-api/internal/domain"
	"sync"
	"testing"
)

type MockRepo struct {
	Data sync.Map
}

type notFoundError struct{}

func (e *notFoundError) Error() string {
	return "Game not found"
}

func (m *MockRepo) Save(game *domain.Session) error {
	m.Data.Store(game.UUID, *game)
	return nil
}

func (m *MockRepo) Load(uuid string) (*domain.Session, error) {
	session, ok := m.Data.Load(uuid)
	if !ok {
		return nil, &notFoundError{}
	}
	s := session.(domain.Session)
	return &s, nil
}

func (m *MockRepo) ListAvailable() ([]domain.Session, error) {
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

func (m *MockRepo) GetCurrentGames(gameUUID, playerUUID string) ([]domain.Session, error) {
	ans := make([]domain.Session, 0)
	m.Data.Range(func(key any, value any) bool {
		s := value.(domain.Session)
		if (s.Player1UUID == playerUUID || s.Player2UUID == playerUUID) && (gameUUID == "" || s.UUID == gameUUID) {
			ans = append(ans, s)
		}
		return true
	})
	return ans, nil
}

func (m *MockRepo) GetUser(uuid string) (*domain.User, error) {
	return nil, nil
}

type MockAuthenticator struct{}

func (a *MockAuthenticator) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), UserUUIDKey, "test-user-uuid")
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func createTestServer(repo domain.GameRepositoryInterface) *httptest.Server {
	service := domain.NewGameService(repo, domain.Cross, domain.Nought)
	gameHandler := NewGameHandler(service)
	auth := &MockAuthenticator{}
	mux := http.NewServeMux()
	mux.Handle("POST /game/{uuid}", auth.Middleware(http.HandlerFunc(gameHandler.PostGame)))
	return httptest.NewServer(mux)
}

func TestConcurrent(t *testing.T) {
	repo := &MockRepo{}
	ts := createTestServer(repo)
	defer ts.Close()

	var wg sync.WaitGroup
	goroutinesNum := 50
	wg.Add(goroutinesNum)

	for i := 0; i < goroutinesNum; i++ {
		go func(id int) {
			defer wg.Done()
			uid := fmt.Sprintf("test-%d", id)
			url := fmt.Sprintf("%s/game/%s", ts.URL, uid)

			// Pre-save game
			repo.Save(&domain.Session{
				UUID:            uid,
				Status:          domain.Turn,
				CurrentTurnUUID: "test-user-uuid",
				Player1UUID:     "test-user-uuid",
				Player1Sign:     domain.Cross,
			})

			field := domain.Field{
				Grid: [3][3]int{{domain.Cross, 0, 0}, {0, 0, 0}, {0, 0, 0}},
			}
			reqBody, _ := json.Marshal(MoveRequest{Field: field.Grid})
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				t.Errorf("Failed to post: %v", err)
				return
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
}

func aiVSai(ts *httptest.Server, repo domain.GameRepositoryInterface, t *testing.T, uuid string) {
	url := fmt.Sprintf("%s/game/%s", ts.URL, uuid)

	// Setup game
	repo.Save(&domain.Session{
		UUID:            uuid,
		Status:          domain.Turn,
		CurrentTurnUUID: "test-user-uuid",
		Player1UUID:     "test-user-uuid",
		Player1Sign:     domain.Cross,
		IsWithBot:       true,
		Player2Sign:     domain.Nought,
	})

	for i := 0; i < 10; i++ {
		s, _ := repo.Load(uuid)
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

	// 3. ConnectGame
	connReq, _ := json.Marshal(ConnectGameRequest{GameUUID: createResp.UUID})
	resp, err = http.Post(ts.URL+"/game/connect", "application/json", bytes.NewBuffer(connReq))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("ConnectGame failed: status %v, err %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// 4. Duplicate ConnectGame (Game already started)
	resp, err = http.Post(ts.URL+"/game/connect", "application/json", bytes.NewBuffer(connReq))
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request on duplicate connect, got status %v", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. GetCurrentGame
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/game/current", bytes.NewBuffer(connReq))
	resp, err = http.DefaultClient.Do(req)
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

func TestWebToDomainMapper(t *testing.T) {
	wm := GameModel{
		UUID:        "g-uuid",
		Field:       [3][3]int{{1, 0, 0}, {0, 2, 0}, {0, 0, 0}},
		Player1UUID: "p1",
		Player2UUID: "p2",
		Status:      1,
		IsWithBot:   false,
	}

	domainSession := wm.WebToDomain()
	if domainSession.UUID != wm.UUID || domainSession.F.Grid != wm.Field {
		t.Errorf("WebToDomain failed: %+v", domainSession)
	}
}

