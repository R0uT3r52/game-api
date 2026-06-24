package di

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"game-api/internal/datasource"
	"game-api/internal/domain"
	"game-api/internal/web"

	"go.uber.org/fx"
)

func NewServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	return mux
}

func RegisterRoute(mux *http.ServeMux, h web.GameHandlerInterface) {
	mux.HandleFunc("POST /game/{uuid}", h.PostGame)
}

func NewHTTPServer(lc fx.Lifecycle, mux *http.ServeMux) *http.Server {
	srv := &http.Server{Addr: ":8080", Handler: mux}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			fmt.Println("Starting HTTP server at", srv.Addr)
			go srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
	return srv
}

func Injection() fx.Option {
	return fx.Provide(
		func() domain.GameRepositoryInterface {
			return datasource.NewGameRepo()
		},
		func(repo domain.GameRepositoryInterface) domain.GameServiceInterface {
			return domain.NewGameService(repo, domain.Cross, domain.Nought)
		},
		func(s domain.GameServiceInterface) web.GameHandlerInterface {
			return web.NewGameHandler(s)
		},
		NewServeMux,
		NewHTTPServer,
	)

}
