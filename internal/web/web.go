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

func (h *GameHandler) SendResponse(w http.ResponseWriter, uuid string, field [3][3]int, winner *int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GameModel{
		UUID:   uuid,
		Field:  field,
		Winner: winner,
	})
}

func (h *GameHandler) PostGame(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")

	var webModel GameModel
	if err := json.NewDecoder(r.Body).Decode(&webModel); err != nil {
		log.Printf("Failed to decode game request body [uuid: %s]: %v", uuid, err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	domainModel := webModel.WebToDomain()

	if err := h.Service.ValidateField(uuid, domainModel.F); err != nil {
		log.Printf("Game field validation failed [uuid: %s]: %v", uuid, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isEnded, winner, err := h.Service.CheckGameEnd(uuid)
	if err != nil {
		var notFoundErr *domain.GameNotFoundError
		if errors.As(err, &notFoundErr) {
			log.Printf("Game not found [uuid: %s]", uuid)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Printf("Failed to check game end [uuid: %s]: %v", uuid, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if isEnded {
		log.Printf("Game ended during check [uuid: %s]: winner %d", uuid, winner)
		h.SendResponse(w, uuid, domainModel.F.Grid, &winner)
		return
	}

	updatedSession, err := h.Service.MakeAiMove(uuid)
	if err != nil {
		log.Printf("Failed to make AI move [uuid: %s]: %v", uuid, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	isEnded, winner, err = h.Service.CheckGameEnd(uuid)
	if err != nil {
		log.Printf("Failed to check game end after AI move [uuid: %s]: %v", uuid, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if isEnded {
		log.Printf("Game ended after AI move [uuid: %s]: winner %d", uuid, winner)
		h.SendResponse(w, uuid, updatedSession.F.Grid, &winner)
		return
	}

	log.Printf("Game move processed successfully [uuid: %s]", uuid)
	h.SendResponse(w, uuid, updatedSession.F.Grid, nil)
}
