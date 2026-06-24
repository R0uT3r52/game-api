package main

import (
	"net/http"
	"game-api/internal/di"

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
