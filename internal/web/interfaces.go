package web

import (
	"net/http"
)

type GameHandlerInterface interface {
	PostGame(w http.ResponseWriter, r *http.Request)
}
