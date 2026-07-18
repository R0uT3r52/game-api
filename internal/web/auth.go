package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"game-api/internal/domain"
)

func NewUserHandler(svc domain.AuthServiceInterface) *UserHandler {
	return &UserHandler{
		Service: svc,
	}
}

func NewUserAuthenticator(svc domain.AuthServiceInterface) *UserAuthenticator {
	return &UserAuthenticator{
		AuthService: svc,
	}
}

// RegisterUser(w http.ResponseWriter, r *http.Request)
// AuthUser(w http.ResponseWriter, r *http.Request)

func (u *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {

	var sReq domain.SignUpRequest

	if err := json.NewDecoder(r.Body).Decode(&sReq); err != nil {
		http.Error(w, "unable to parse request data", 400)
		log.Print(err)
		return
	}

	if err := u.Service.Register(sReq); err != nil {
		http.Error(w, "unable to register user", 500)
		log.Print(err)
		return
	}

}

func (u *UserHandler) AuthUser(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")

	uuid, err := u.Service.Authorize(auth)
	if err != nil {
		w.WriteHeader(401)
		log.Print(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		UUID string `json:"uuid"`
	}{
		UUID: uuid,
	})
}

func (a *UserAuthenticator) Middleware(h http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		uuid, err := a.AuthService.Authorize(auth)
		if err != nil {
			w.WriteHeader(401)
			log.Print(err)
			return
		}

		ctx := context.WithValue(r.Context(), "user_uuid", uuid)
		h.ServeHTTP(w, r.WithContext(ctx))
	})

}

// func (h *UserHandler) SendResponse(w http.ResponseWriter, uuid string, field [3][3]int, winner *int) {
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(GameModel{
// 		UUID:   uuid,
// 		Field:  field,
// 		Winner: winner,
// 	})
// }
