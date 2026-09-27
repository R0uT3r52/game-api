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

type contextKey string

const UserUUIDKey contextKey = "user_uuid"

func UserUUIDFromContext(ctx context.Context) string {
	uuid, _ := ctx.Value(UserUUIDKey).(string)
	return uuid
}

func NewUserHandler(svc domain.AuthServiceInterface) *UserHandler {
	return &UserHandler{
		Service: svc,
	}
}

func NewUserAuthenticator(jwtp domain.JwtProviderInterface) *UserAuthenticator {
	return &UserAuthenticator{
		JwtProvider: jwtp,
	}
}

func (u *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var sReq domain.SignUpRequest

	if err := json.NewDecoder(r.Body).Decode(&sReq); err != nil {
		log.Printf("Failed to decode signup request: %v", err)
		http.Error(w, "Unable to parse request data", http.StatusBadRequest)
		return
	}

	if err := u.Service.Register(r.Context(), sReq); err != nil {
		var existsErr *domain.UserAlreadyExistsError
		var invalidCredsErr *domain.ValidationError
		if errors.As(err, &existsErr) {
			log.Printf("User registration failed: user already exists: %s", sReq.Login)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.As(err, &invalidCredsErr) {
			log.Printf("User registration failed: invalid user credentials: %s", sReq.Login)
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

func (u *UserHandler) UpdateAccessToken(w http.ResponseWriter, r *http.Request) {
	var refReq RefreshJwtRequest

	if err := json.NewDecoder(r.Body).Decode(&refReq); err != nil {
		log.Printf("Failed to decode refresh request: %v", err)
		http.Error(w, "Unable to parse request data", http.StatusBadRequest)
		return
	}

	jwtResp, err := u.Service.UpdateAccessToken(r.Context(), refReq.RefreshToken)
	if err != nil {
		var invalidToken *domain.InvalidTokenError
		var incCreds *domain.IncorrectCredsError
		if errors.As(err, &invalidToken) {
			log.Printf("Invalid token to refresh")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if errors.As(err, &incCreds) {
			log.Printf("Incorrect creds in access token refresh")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		log.Printf("Internal error updating access token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwtResp)
}

func (u *UserHandler) UpdateRefreshToken(w http.ResponseWriter, r *http.Request) {
	var refReq RefreshJwtRequest

	if err := json.NewDecoder(r.Body).Decode(&refReq); err != nil {
		log.Printf("Failed to decode refresh request: %v", err)
		http.Error(w, "Unable to parse request data", http.StatusBadRequest)
		return
	}

	jwtResp, err := u.Service.UpdateRefreshToken(r.Context(), refReq.RefreshToken)
	if err != nil {
		var invalidToken *domain.InvalidTokenError
		var incCreds *domain.IncorrectCredsError
		if errors.As(err, &invalidToken) {
			log.Printf("Invalid token to refresh")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if errors.As(err, &incCreds) {
			log.Printf("Incorrect creds in refresh token refresh")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		log.Printf("Internal error updating refresh token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwtResp)
}

func (u *UserHandler) AuthUser(w http.ResponseWriter, r *http.Request) {

	var jwtReq domain.JwtRequest

	if err := json.NewDecoder(r.Body).Decode(&jwtReq); err != nil {
		log.Printf("Failed to decode jwt request body: %v", err)
		http.Error(w, "incorrect request body", http.StatusBadRequest)
		return
	}

	jwtResp, err := u.Service.Authorize(r.Context(), &jwtReq)
	if err != nil {
		var incorrectCreds *domain.IncorrectCredsError
		if errors.As(err, &incorrectCreds) {
			log.Printf("Unauthorized login attempt: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Internal error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("User authorized successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwtResp)
}

func (u *UserHandler) GetUserByToken(w http.ResponseWriter, r *http.Request) {
	uuid, ok := r.Context().Value(UserUUIDKey).(string)
	if !ok || len(uuid) == 0 {
		log.Printf("Unable to get uuid from context")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, err := u.Service.GetUser(r.Context(), uuid)
	if err != nil || user == nil {
		log.Printf("Failed to get user [uuid: %s]: %v", uuid, err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserResponse{
		UUID:  user.UUID,
		Login: user.Login,
	})
}

func (u *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userUUID := r.PathValue("uuid")

	if userUUID == "" {
		log.Printf("Failed to get empty user")
		http.Error(w, "incorrect user uuid", http.StatusBadRequest)
		return
	}

	user, err := u.Service.GetUser(r.Context(), userUUID)
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

		authHeader := r.Header.Get("Authorization")
		if len(authHeader) == 0 {
			log.Printf("Unauthorized request to protected endpoint %s", r.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			log.Printf("Authorization without bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tkn := strings.TrimPrefix(authHeader, "Bearer ")

		err := a.JwtProvider.ValidateAccessToken(tkn)
		if err != nil {
			log.Printf("Invalid access token in Middleware: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		uuid, err := a.JwtProvider.GetUUIDFromToken(tkn)
		if err != nil {
			log.Printf("Invalid access token in Middleware: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserUUIDKey, uuid)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
