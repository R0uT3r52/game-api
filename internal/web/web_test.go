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
				t.Errorf("Expected not 200 OK status")
			}
			defer resp.Body.Close()
		}(i)
	}
	wg.Wait()
}

func TestAIvsAI(t *testing.T) {
	ts := createTestServer()
	defer ts.Close()

	clientRepo := datasource.NewGameRepo()
	clientSvc := domain.NewGameService(clientRepo, domain.Nought, domain.Cross)

	uuid := "test"
	url := fmt.Sprintf("%s/game/%s", ts.URL, uuid)

	currentField := domain.Field{}

	for i := 0; i < 10; i++ {
		err := clientRepo.Save(&domain.Session{UUID: uuid, F: currentField})
		if err != nil {
			t.Fatalf("Failed to save to client repo: %v", err)
		}

		updatedSession, err := clientSvc.MakeAiMove(uuid)
		if err != nil {
			t.Fatalf("Client AI failed to make move: %v", err)
		}

		reqBody, _ := json.Marshal(GameModel{
			UUID:  uuid,
			Field: updatedSession.F.Grid,
		})

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			t.Fatalf("Failed to post game: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			resp.Body.Close()
			t.Fatalf("Expected status 200, got %v. Response: %s", resp.StatusCode, buf.String())
		}

		var gameResp GameModel
		if err := json.NewDecoder(resp.Body).Decode(&gameResp); err != nil {
			resp.Body.Close()
			t.Fatalf("Failed to decode response: %v", err)
		}
		resp.Body.Close()

		if gameResp.Winner != nil {
			if *gameResp.Winner != domain.Tie {
				t.Errorf("Tie expected between two AI. Winner: %v", *gameResp.Winner)
			}
			return // Game ended correctly
		}

		// update field for next move
		currentField.Grid = gameResp.Field
	}

	t.Errorf("game not ended correctly")
}
