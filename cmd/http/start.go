package http

import (
	"fmt"
	"net/http"

	"github.com/bnkamalesh/phispr/internal/api"
	"github.com/bnkamalesh/phispr/internal/configs"
	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"
	"github.com/naughtygopher/webgo/v7/middleware/accesslog"
	"github.com/naughtygopher/webgo/v7/middleware/cors"
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
			Name:          "join-room",
			Method:        http.MethodPost,
			Pattern:       "/rooms/:roomID",
			Handlers:      []http.HandlerFunc{ht.JoinRoomHandler},
			TrailingSlash: true,
		},
		{
			Name:          "room-subscription",
			Method:        http.MethodGet,
			Pattern:       "/rooms/:roomID/messages",
			Handlers:      []http.HandlerFunc{ht.SSEHandler},
			TrailingSlash: true,
		},
		{
			Name:          "send-message",
			Method:        http.MethodPost,
			Pattern:       "/rooms/:roomID/messages",
			Handlers:      []http.HandlerFunc{ht.NewMessage},
			TrailingSlash: true,
		},
	}
}

func setup(cfg *configs.Config) *webgo.Router {
	wcfg := &webgo.Config{
		Host:         cfg.HTTP.Host,
		Port:         fmt.Sprintf("%d", cfg.HTTP.Port),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	webgo.GlobalLoggerConfig(nil, nil, webgo.LogCfgDisableDebug)

	api := api.NewAPI(rooms.NewRooms(cfg.Rooms.Capacity))
	ht := HTTP{
		api:          api,
		sse:          sse.New(),
		templateHome: loadTemplate(cfg.HTTP.TemplateHome, "home"),
		templateRoom: loadTemplate(cfg.HTTP.TemplateRoom, "room"),
		templateErr:  loadTemplate(cfg.HTTP.TemplateError, "error"),
	}
	routes := getRoutes(&ht)

	router := webgo.NewRouter(wcfg, routes...)
	router.Use(
		cors.CORS(&cors.Config{
			AllowedOrigins: cfg.HTTP.AllowedOrigins,
			AllowedHeaders: cfg.HTTP.AllowedHeaders,
		}),
	)

	if cfg.HTTP.EnableAccessLog {
		router.Use(accesslog.AccessLog)
	}

	return router
}

func Start() {
	cfg := configs.Load("./config.yaml")
	router := setup(cfg)

	errTmpl := loadTemplate(cfg.HTTP.TemplateError, "error")
	router.NotFound = func(w http.ResponseWriter, r *http.Request) {
		errorHandler(errTmpl, w, errors.NotFoundf(`Here, take your URL back please (つ•᷄᎑•᷅)  %q.`, r.URL.Path))
	}

	router.Start()
}
