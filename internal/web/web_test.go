package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"game-api/internal/datasource"
	"game-api/internal/domain"
	"sync"
	"testing"
)

func createTestServer() *httptest.Server {
	repo := datasource.NewGameRepo()
	service := domain.NewGameService(repo, domain.Cross, domain.Nought)
	gameHandler := NewGameHandler(service)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /game/{uuid}", gameHandler.PostGame)

	return httptest.NewServer(mux)
}

func TestConcurrent(t *testing.T) {
	ts := createTestServer()
	defer ts.Close()

	var wg sync.WaitGroup
	goroutinesNum := 1000
	wg.Add(goroutinesNum)

	for i := range goroutinesNum {
		go func(id int) {
			defer wg.Done()

			uid := fmt.Sprintf("test-%d", i)
			url := fmt.Sprintf("%s/game/%s", ts.URL, uid)

			field := domain.Field{
				Grid: [3][3]int{
					{domain.Cross, domain.Empty, domain.Empty},
					{domain.Empty, domain.Empty, domain.Empty},
					{domain.Empty, domain.Empty, domain.Empty},
				},
			}

			reqBody, _ := json.Marshal(GameModel{
				UUID:  uid,
				Field: field.Grid,
			})

			resp, _ := http.Post(url, "application/json", bytes.NewBuffer(reqBody))

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected 200 OK status")
			}
			defer resp.Body.Close()
		}(i)
	}
	wg.Wait()
}

func aiVSai(ts *httptest.Server, t *testing.T, uuid string) {
	url := fmt.Sprintf("%s/game/%s", ts.URL, uuid)

	clientRepo := datasource.NewGameRepo()
	clientSvc := domain.NewGameService(clientRepo, domain.Nought, domain.Cross)

	currentField := domain.Field{}

	for i := 0; i < 10; i++ {
		err := clientRepo.Save(&domain.Session{UUID: uuid, F: currentField})
		if err != nil {
			t.Errorf("[UUID: %s] Failed to save to client repo: %v", uuid, err)
			return
		}

		updatedSession, err := clientSvc.MakeAiMove(uuid)
		if err != nil {
			t.Errorf("[UUID: %s] Client AI failed to make move: %v", uuid, err)
			return
		}

		reqBody, _ := json.Marshal(GameModel{
			UUID:  uuid,
			Field: updatedSession.F.Grid,
		})

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			t.Errorf("[UUID: %s] Failed to post game: %v", uuid, err)
			return
		}

		if resp.StatusCode != http.StatusOK {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			t.Errorf("[UUID: %s] Expected status 200, got %v. Response: %s", uuid, resp.StatusCode, buf.String())
			return
		}

		var gameResp GameModel
		if err := json.NewDecoder(resp.Body).Decode(&gameResp); err != nil {
			resp.Body.Close()
			t.Errorf("[UUID: %s] Failed to decode response: %v", uuid, err)
			return
		}
		resp.Body.Close()

		if gameResp.Winner != nil {
			if *gameResp.Winner != domain.Tie {
				t.Errorf("[UUID: %s] Tie expected between two AI. Winner: %v", uuid, *gameResp.Winner)
			}
			return // Game ended correctly
		}

		// update field for next move
		currentField.Grid = gameResp.Field
	}

	t.Errorf("[UUID: %s] game not ended correctly", uuid)

}

func TestAIvsAI(t *testing.T) {
	ts := createTestServer()
	defer ts.Close()

	uuid := "test"

	aiVSai(ts, t, uuid)
}

func TestConcurrentAIvsAI(t *testing.T) {
	ts := createTestServer()
	defer ts.Close()
	var wg sync.WaitGroup
	gamesCount := 20
	wg.Add(gamesCount)
	for i := 0; i < gamesCount; i++ {
		go func(id int) {
			defer wg.Done()

			uuid := fmt.Sprintf("concurrent-game-%d", id)

			aiVSai(ts, t, uuid)

		}(i)
	}
	wg.Wait()
}
