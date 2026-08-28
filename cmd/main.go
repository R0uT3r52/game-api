package main

import (
	"game-api/internal/di"
	"net/http"

	"go.uber.org/fx"
)

func main() {
	opts := di.Injection()
	fx.New(
		opts,
		fx.Invoke(
			di.RegisterRoute,
			func(*http.Server) {},
		),
	).Run()

}
