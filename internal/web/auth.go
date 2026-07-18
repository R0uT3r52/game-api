package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"game-api/internal/domain"
	"strings"
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

func (u *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var sReq domain.SignUpRequest

	if err := json.NewDecoder(r.Body).Decode(&sReq); err != nil {
		log.Printf("Failed to decode signup request: %v", err)
		http.Error(w, "Unable to parse request data", http.StatusBadRequest)
		return
	}

	if err := u.Service.Register(sReq); err != nil {
		var existsErr *domain.UserAlreadyExistsError
		if errors.As(err, &existsErr) {
			log.Printf("User registration failed: user already exists: %s", sReq.Login)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("User registration failed due to internal error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("User registered successfully: %s", sReq.Login)
	w.WriteHeader(http.StatusCreated)
}

func (u *UserHandler) AuthUser(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")

	uuid, err := u.Service.Authorize(auth)
	if err != nil {
		var incorrectCreds *domain.IncorrectCredsError
		if errors.As(err, &incorrectCreds) || strings.Contains(err.Error(), "auth header") {
			log.Printf("Unauthorized login attempt: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Printf("User login failed due to internal error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("User authorized successfully: %s", uuid)

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
			log.Printf("Unauthorized request to protected endpoint %s: %v", r.URL.Path, err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Printf("Request authorized for %s: user %s", r.URL.Path, uuid)

		ctx := context.WithValue(r.Context(), "user_uuid", uuid)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
