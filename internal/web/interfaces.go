package web

import (
	"net/http"
)

type HandlerInterface interface {
	PostGame(w http.ResponseWriter, r *http.Request)
}
