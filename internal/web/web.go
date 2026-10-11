package web

import (
	"encoding/json"
	"errors"
	"game-api/internal/domain"
	"log"
	"net/http"
	"strconv"
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

	uuid, err := h.Service.CreateGame(r.Context(), creatorUUID, createGameRequest.IsWithBot)
	if err != nil {
		log.Printf("Failed to create game: %v", err)
		http.Error(w, "unable to create game", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(TokenResponse{UUID: uuid}); err != nil {
		log.Printf("Failed to encode create game response: %v", err)
	}
}

func (h *GameHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	ans, err := h.Service.GetAvailableGames(r.Context())
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
	gameUUID := r.PathValue("uuid")

	reqUUID := UserUUIDFromContext(r.Context())

	sessions, err := h.Service.GetCurrentGames(r.Context(), gameUUID, reqUUID)

	var e *domain.GameNotFoundError
	if err != nil && errors.As(err, &e) {
		log.Printf("Game not found with UUID: %s", gameUUID)
		http.Error(w, "Game not found with this uuid", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("Failed to get active games [gameUUID: %s, userUUID: %s]: %v", gameUUID, reqUUID, err)
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
	if err := json.NewEncoder(w).Encode(webModels); err != nil {
		log.Printf("Failed to encode current game response: %v", err)
	}
}

func (h *GameHandler) GetFinishedGames(w http.ResponseWriter, r *http.Request) {

	reqCtx := r.Context()
	playerUUID := UserUUIDFromContext(reqCtx)

	sessions, err := h.Service.GetFinishedGames(reqCtx, playerUUID)
	if err != nil {
		var incCreds *domain.IncorrectCredsError
		if errors.As(err, &incCreds) {
			log.Printf("Incorrect playerUUID")
			http.Error(w, "Unauthorized playerUUID", http.StatusUnauthorized)
			return
		} else {
			log.Printf("Internal error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	ans := make([]GameModel, len(sessions))
	for i := range ans {
		ans[i].DomainToWeb(&sessions[i])
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ans); err != nil {
		log.Printf("Failed to encode finished games response: %v", err)
	}
}

func (h *GameHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {

	queryParams := r.URL.Query()

	limitq := queryParams.Get("limit")
	if limitq == "" {
		limitq = queryParams.Get("n")
	}
	if limitq == "" {
		limitq = queryParams.Get("N")
	}
	if limitq == "" {
		limitq = "50"
	}
	limit, err := strconv.ParseInt(limitq, 10, 32)

	if err != nil {
		log.Printf("Incorrect limit value in query: %v", err)
		http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
		return
	}

	result, err := h.Service.GetLeaderboard(r.Context(), limit)
	if err != nil {
		if errors.Is(err, domain.ErrIncorrectLimit) {
			log.Printf("Incorrect limit value: %d", limit)
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		} else {
			log.Printf("Internal error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	ans := make([]LeaderBoardModel, len(result))

	for i := range result {
		ans[i].DomainToWeb(&result[i])
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ans); err != nil {
		log.Printf("Failed to encode leaderboard response: %v", err)
	}
}

func (h *GameHandler) ConnectGame(w http.ResponseWriter, r *http.Request) {

	var req ConnectGameRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode create game request body: %v", err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	reqUUID := UserUUIDFromContext(r.Context())

	err := h.Service.Connect(r.Context(), reqUUID, req.GameUUID)
	if err != nil {
		if errors.Is(err, domain.ErrGameAlreadyStarted) {
			log.Printf("Failed to connect to game [uuid: %s]: %v", req.GameUUID, err)
			http.Error(w, "unable to connect (game already started)", http.StatusBadRequest)
			return
		}
		if errors.Is(err, domain.ErrUserAlreadyInGame) {
			log.Printf("Failed to connect to game [uuid: %s]: %v", req.GameUUID, err)
			http.Error(w, "unable to connect (user is already in this game)", http.StatusBadRequest)
			return
		}
		log.Printf("Failed to connect to game [uuid: %s]: %v", req.GameUUID, err)
		http.Error(w, "unable to connect to the game", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode("connected successfully"); err != nil {
		log.Printf("Failed to encode connect response: %v", err)
	}
}

func (h *GameHandler) SendResponse(w http.ResponseWriter, session *domain.Session) {
	w.Header().Set("Content-Type", "application/json")
	var webModel GameModel
	webModel.DomainToWeb(session)
	if err := json.NewEncoder(w).Encode(webModel); err != nil {
		log.Printf("Failed to encode game response: %v", err)
	}
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

	updatedSession, err := h.Service.MakeMove(r.Context(), gameUUID, playerUUID, domain.Field{Grid: moveReq.Field})
	if err != nil {
		var valErr *domain.ValidationError
		if errors.As(err, &valErr) {
			log.Printf("Move validation failed [uuid: %s]: %v", gameUUID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(ErrorResponse{Message: valErr.Message}); err != nil {
				log.Printf("Failed to encode error response: %v", err)
			}
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
