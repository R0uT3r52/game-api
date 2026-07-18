package web

import (
	"net/http"
)

type GameHandlerInterface interface {
	PostGame(w http.ResponseWriter, r *http.Request)
}

type UserHandlerInterface interface {
	RegisterUser(w http.ResponseWriter, r *http.Request)
	AuthUser(w http.ResponseWriter, r *http.Request)
}

type UserAuthenticatorInterface interface {
	Middleware(h http.Handler) http.Handler
}
