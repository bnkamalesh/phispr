package http

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bnkamalesh/chat/internal/api"
	"github.com/bnkamalesh/chat/internal/rooms"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"
	"github.com/naughtygopher/webgo/v7/middleware/accesslog"
	"github.com/naughtygopher/webgo/v7/middleware/cors"
)

var (
	lastModified = time.Now().Format(http.TimeFormat)
)

func getRoutes(ht *HTTP) []*webgo.Route {
	return []*webgo.Route{
		{
			Name:          "static",
			Method:        http.MethodGet,
			Pattern:       "/static/:w*",
			Handlers:      []http.HandlerFunc{ht.StaticFilesHandler},
			TrailingSlash: true,
		},
		{
			Name:          "root",
			Method:        http.MethodGet,
			Pattern:       "/",
			Handlers:      []http.HandlerFunc{ht.HomeHandler},
			TrailingSlash: true,
		},
		{
			Name:                    "create-and-join",
			Method:                  http.MethodPost,
			Pattern:                 "/rooms",
			Handlers:                []http.HandlerFunc{ht.CreateJoinRoomHandler},
			TrailingSlash:           true,
			FallThroughPostResponse: true,
		},
		{
			Name:                    "room",
			Method:                  http.MethodGet,
			Pattern:                 "/rooms/:roomID",
			Handlers:                []http.HandlerFunc{ht.RoomHandler},
			TrailingSlash:           true,
			FallThroughPostResponse: true,
		},
		{
			Name:          "join room",
			Method:        http.MethodPost,
			Pattern:       "/rooms/:roomID",
			Handlers:      []http.HandlerFunc{ht.JoinRoomHandler},
			TrailingSlash: true,
		},
		{
			Name:          "room subscription",
			Method:        http.MethodGet,
			Pattern:       "/rooms/:roomID/messages",
			Handlers:      []http.HandlerFunc{ht.SSEHandler()},
			TrailingSlash: true,
		},
		{
			Name:          "send message",
			Method:        http.MethodPost,
			Pattern:       "/rooms/:roomID/messages",
			Handlers:      []http.HandlerFunc{ht.NewMessage},
			TrailingSlash: true,
		},
	}
}

func setup(env string) (*webgo.Router, *sse.SSE) {
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = "8080"
	}
	cfg := &webgo.Config{
		Host:         "",
		Port:         port,
		HTTPSPort:    "9595",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 1 * time.Hour,
		CertFile:     "./certs/localhost.crt",
		KeyFile:      "./certs/localhost.decrypted.key",
	}

	webgo.GlobalLoggerConfig(
		nil, nil,
		webgo.LogCfgDisableDebug,
	)

	sseService := sse.New()

	api := api.NewAPI(rooms.NewRooms(100))
	ht := HTTP{
		api:          api,
		sse:          sseService,
		templateHome: templateHomepage(),
		templateRoom: templateRoom(),
		templateErr:  templateError(),
	}
	routes := getRoutes(&ht)

	router := webgo.NewRouter(cfg, routes...)
	router.Use(
		cors.CORS(&cors.Config{
			AllowedOrigins: []string{"chat.maakri.space"},
			AllowedHeaders: []string{"*"},
		}),
	)
	if env == "dev" {
		router.Use(accesslog.AccessLog)
	}

	return router, sseService
}

func Start() {
	router, sseService := setup("dev")
	clients := []*sse.Client{}
	sseService.OnCreateClient = func(ctx context.Context, client *sse.Client, count int) {
		clients = append(clients, client)
	}

	errTmpl := templateError()
	router.NotFound = func(w http.ResponseWriter, r *http.Request) {
		errorHandler(errTmpl, w, errors.NotFoundf(`Here, take your URL back please (つ•᷄᎑•᷅)  %q.`, r.URL.Path))
	}

	go router.StartHTTPS()
	router.Start()
}
