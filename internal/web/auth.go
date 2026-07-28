package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"game-api/internal/domain"
	"strings"
)

type contextKey string

const UserUUIDKey contextKey = "user_uuid"

func parseBasicAuthHeader(header string) (login, password string, err error) {
	data := header

	if len(header) >= 6 && strings.EqualFold(header[:6], "basic ") {
		data = header[6:]
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", "", errors.New("auth header decode error")
	}

	creds := strings.SplitN(string(decodedBytes), ":", 2)
	if len(creds) != 2 {
		return "", "", errors.New("auth header: invalid credentials format")
	}

	return creds[0], creds[1], nil
}

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

	login, password, err := parseBasicAuthHeader(auth)
	if err != nil {
		log.Printf("Unauthorized login attempt: %v", err)
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	uuid, err := u.Service.Authorize(login, password)
	if err != nil {
		var incorrectCreds *domain.IncorrectCredsError
		if errors.As(err, &incorrectCreds) {
			log.Printf("Unauthorized login attempt: %v", err)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

func (u *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userUUID := r.PathValue("uuid")

	if userUUID == "" {
		log.Printf("Failed to get empty user")
		http.Error(w, "incorrect user uuid", http.StatusBadRequest)
		return
	}

	_, ok := r.Context().Value(UserUUIDKey).(string)
	if !ok {
		log.Printf("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := u.Service.GetUser(userUUID)
	if err != nil || user == nil {
		log.Printf("Failed to get user [uuid: %s]: %v", userUUID, err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserResponse{
		UUID:  user.UUID,
		Login: user.Login,
	})
}

func (a *UserAuthenticator) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		login, password, err := parseBasicAuthHeader(auth)
		if err != nil {
			log.Printf("Unauthorized request to protected endpoint %s: %v", r.URL.Path, err)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		uuid, err := a.AuthService.Authorize(login, password)
		if err != nil {
			log.Printf("Unauthorized request to protected endpoint %s: %v", r.URL.Path, err)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Request authorized for %s: user %s", r.URL.Path, uuid)

		ctx := context.WithValue(r.Context(), UserUUIDKey, uuid)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
