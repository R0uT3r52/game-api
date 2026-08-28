package di

import (
	"context"
	"fmt"
	"game-api/internal/datasource"
	"game-api/internal/domain"
	"game-api/internal/web"
	"net"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

func NewServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	return mux
}

func RegisterRoute(mux *http.ServeMux, h web.GameHandlerInterface, u web.UserHandlerInterface, a web.UserAuthenticatorInterface) {

	authPostGame := http.HandlerFunc(h.PostGame)
	authCreateGame := http.HandlerFunc(h.CreateGame)
	authListGames := http.HandlerFunc(h.ListGames)
	authConnectGame := http.HandlerFunc(h.ConnectGame)
	authGetCurrentGame := http.HandlerFunc(h.GetCurrentGame)
	authGetUser := http.HandlerFunc(u.GetUser)

	mux.Handle("POST /game/{uuid}", a.Middleware(authPostGame))
	mux.Handle("POST /game/new", a.Middleware(authCreateGame))
	mux.Handle("GET /games/available", a.Middleware(authListGames))
	mux.Handle("POST /game/connect", a.Middleware(authConnectGame))
	mux.Handle("GET /game/current", a.Middleware(authGetCurrentGame))
	mux.Handle("GET /game/current/{uuid}", a.Middleware(authGetCurrentGame)) // uuid as an optional path parameter
	mux.Handle("GET /user/{uuid}", a.Middleware(authGetUser))

	mux.HandleFunc("POST /signup", u.RegisterUser)
	mux.HandleFunc("POST /login", u.AuthUser)
}

func NewHTTPServer(lc fx.Lifecycle, mux *http.ServeMux) *http.Server {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: mux}
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
		datasource.GetDB,
		func(db *pgxpool.Pool) domain.GameRepositoryInterface {
			return datasource.NewGameRepo(db)
		},
		func(repo domain.GameRepositoryInterface) domain.GameServiceInterface {
			return domain.NewGameService(repo, domain.Cross, domain.Nought)
		},
		func(s domain.GameServiceInterface) web.GameHandlerInterface {
			return web.NewGameHandler(s)
		},
		func(db *pgxpool.Pool) domain.UserServiceInterface {
			return &datasource.UserRepository{Data: db}
		},
		func(uSvc domain.UserServiceInterface) domain.AuthServiceInterface {
			return &domain.AuthService{UserSvc: uSvc}
		},
		func(svc domain.AuthServiceInterface) web.UserHandlerInterface {
			return web.NewUserHandler(svc)
		},
		func(svc domain.AuthServiceInterface) web.UserAuthenticatorInterface {
			return web.NewUserAuthenticator(svc)
		},
		NewServeMux,
		NewHTTPServer,
	)

}
