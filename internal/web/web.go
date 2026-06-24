package web

import (
	"encoding/json"
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
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	domainModel := webModel.WebToDomain()

	if err := h.Service.ValidateField(uuid, domainModel.F); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isEnded, winner, err := h.Service.CheckGameEnd(uuid)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isEnded {
		h.SendResponse(w, uuid, domainModel.F.Grid, &winner)
		return
	}

	updatedSession, err := h.Service.MakeAiMove(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isEnded, winner, err = h.Service.CheckGameEnd(uuid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isEnded {
		h.SendResponse(w, uuid, updatedSession.F.Grid, &winner)
		return
	}

	h.SendResponse(w, uuid, updatedSession.F.Grid, nil)
}
