package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"game-api/internal/domain"
)

func NewGameHandler(s domain.GameServiceInterface) *GameHandler {
	return &GameHandler{
		Service: s,
	}
}

func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	var createGameRequest CreateGameRequest

	if err := json.NewDecoder(r.Body).Decode(&createGameRequest); err != nil {
		log.Printf("Failed to decode game create request body: %v", err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	creatorUUID := UserUUIDFromContext(r.Context())

	uuid, err := h.Service.CreateGame(creatorUUID, createGameRequest.IsWithBot)
	if err != nil {
		log.Printf("Failed to create game: %v", err)
		http.Error(w, "unable to create game", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(TokenResponse{UUID: uuid})
}

func (h *GameHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	ans, err := h.Service.GetAvailableGames()
	if err != nil {
		log.Printf("Failed to list games: %v", err)
		http.Error(w, "unable to list games", http.StatusInternalServerError)
		return
	}

	var webModels []GameModel
	for _, s := range ans {
		var m GameModel
		m.DomainToWeb(&s)
		webModels = append(webModels, m)
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(webModels); err != nil {
		log.Printf("Failed to encode games: %v", err)
		http.Error(w, "failed to send games", http.StatusInternalServerError)
		return
	}
}

func (h *GameHandler) GetCurrentGame(w http.ResponseWriter, r *http.Request) {
	var req ConnectGameRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode game request body: %v", err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	reqUUID := UserUUIDFromContext(r.Context())

	sessions, err := h.Service.GetCurrentGames(req.GameUUID, reqUUID)
	if err != nil {
		log.Printf("Failed to get active games [gameUUID: %s, userUUID: %s]: %v", req.GameUUID, reqUUID, err)
		http.Error(w, "incorrect gameUUID", http.StatusBadRequest)
		return
	}

	var webModels []GameModel
	for _, s := range sessions {
		var m GameModel
		m.DomainToWeb(&s)
		webModels = append(webModels, m)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(webModels)
}

func (h *GameHandler) ConnectGame(w http.ResponseWriter, r *http.Request) {

	var req ConnectGameRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode create game request body: %v", err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	reqUUID := UserUUIDFromContext(r.Context())

	err := h.Service.Connect(reqUUID, req.GameUUID)
	if err != nil {
		if errors.Is(err, domain.ErrGameAlreadyStarted) {
			log.Printf("Failed to connect to game [uuid: %s]: %v", req.GameUUID, err)
			http.Error(w, "unable to connect (game already started)", http.StatusBadRequest)
			return
		}
		log.Printf("Failed to connect to game [uuid: %s]: %v", req.GameUUID, err)
		http.Error(w, "unable to connect to the game", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("connected successfully")
}

func (h *GameHandler) SendResponse(w http.ResponseWriter, session *domain.Session) {
	w.Header().Set("Content-Type", "application/json")
	var webModel GameModel
	webModel.DomainToWeb(session)
	json.NewEncoder(w).Encode(webModel)
}

func (h *GameHandler) PostGame(w http.ResponseWriter, r *http.Request) {
	gameUUID := r.PathValue("uuid")

	playerUUID := UserUUIDFromContext(r.Context())

	var moveReq MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&moveReq); err != nil {
		log.Printf("Failed to decode move request body [uuid: %s]: %v", gameUUID, err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	updatedSession, err := h.Service.MakeMove(gameUUID, playerUUID, domain.Field{Grid: moveReq.Field})
	if err != nil {
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			log.Printf("Move validation failed [uuid: %s]: %v", gameUUID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Message: valErr.Message})
			return
		}

		var notFoundErr *domain.GameNotFoundError
		if errors.As(err, &notFoundErr) {
			log.Printf("Game not found [uuid: %s]", gameUUID)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		log.Printf("Failed to process move [uuid: %s]: %v", gameUUID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("Move processed successfully [uuid: %s]", gameUUID)
	h.SendResponse(w, updatedSession)
}
