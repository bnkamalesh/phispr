// install https://github.com/evanw/esbuild for code generation
//
//go:generate esbuild static/js/common.js --minify --outfile=static/js/min/common.js
//go:generate esbuild static/js/room.js --minify --outfile=static/js/min/room.js
//go:generate esbuild static/js/home.js --minify --outfile=static/js/min/home.js
//go:generate esbuild static/js/sse.js --minify --outfile=static/js/min/sse.js
//go:generate esbuild static/js/serviceworker.js --minify --outfile=static/js/min/serviceworker.js
//go:generate esbuild static/css/main.css --minify --outfile=static/css/min/main.css
//go:generate esbuild static/css/normalize.css --minify --outfile=static/css/min/normalize.css
//go:generate esbuild static/css/themes.css --minify --outfile=static/css/min/themes.css
package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"
	"github.com/naughtygopher/webgo/v7/middleware/accesslog"
	"github.com/naughtygopher/webgo/v7/middleware/cors"

	"github.com/bnkamalesh/phispr/internal/api"
	"github.com/bnkamalesh/phispr/internal/configs"
	"github.com/bnkamalesh/phispr/internal/rooms"
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
			Name:          "static-asset-version",
			Method:        http.MethodGet,
			Pattern:       "/static-asset-version",
			Handlers:      []http.HandlerFunc{ht.StaticAssetVersion},
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
			Name:          "leave-room",
			Method:        http.MethodPost,
			Pattern:       "/rooms/:roomID/leave",
			Handlers:      []http.HandlerFunc{ht.LeaveRoomHandler},
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
		{
			Name:          "boot-user",
			Method:        http.MethodDelete,
			Pattern:       "/rooms/:roomID/:userID",
			Handlers:      []http.HandlerFunc{ht.authOwner(ht.BootUserHandler)},
			TrailingSlash: true,
		},
	}
}

func initServices(cfg *configs.Config) (*rooms.Rooms, *HTTP) {
	rms := rooms.NewRooms(cfg.Rooms.Capacity, cfg.Rooms.MemberCapacity, cfg.Rooms.IdleRoomExpiry)
	if cfg.Rooms.BackupFilePath != "" {
		f, err := os.OpenFile(cfg.Rooms.BackupFilePath, os.O_RDONLY, os.ModePerm)
		if err != nil {
			panic(err)
		}
		payload, err := io.ReadAll(f)
		if err != nil {
			panic(err)
		}
		if len(payload) > 1 {
			err = rms.LoadRoomsBackup(payload)
			if err != nil {
				panic(err)
			}
		}
	}

	api := api.NewAPI(rms)
	ht := &HTTP{
		api:             api,
		sse:             sse.New(),
		staticRoot:      cfg.HTTP.StaticRoot,
		templateHome:    loadTemplate(cfg.HTTP.TemplateHome, "home", cfg.HTTP.LiveReloadTemplate),
		templateRoom:    loadTemplate(cfg.HTTP.TemplateRoom, "room", cfg.HTTP.LiveReloadTemplate),
		templateErr:     loadTemplate(cfg.HTTP.TemplateError, "error", cfg.HTTP.LiveReloadTemplate),
		roomLiveViewers: sync.Map{},
		broadcastDelay:  cfg.Rooms.LiveViewerBroadcastDelay,
	}
	ht.Sanitize()

	return rms, ht
}

func setupRouter(
	cfg *configs.Config,
	ht *HTTP,
) *webgo.Router {
	wcfg := &webgo.Config{
		Host:         cfg.HTTP.Host,
		Port:         fmt.Sprintf("%d", cfg.HTTP.Port),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	webgo.GlobalLoggerConfig(nil, nil, webgo.LogCfgDisableDebug)
	routes := getRoutes(ht)
	router := webgo.NewRouter(wcfg, routes...)
	router.Use(
		func(w http.ResponseWriter, r *http.Request, hf http.HandlerFunc) {
			// this header is required for service worker to be able to
			// cache responses appropriate to negotiated content types
			w.Header().Add("Vary", "Content-type")
			hf(w, r)
		},
		cors.CORS(&cors.Config{
			AllowedOrigins: cfg.HTTP.AllowedOrigins,
			AllowedHeaders: cfg.HTTP.AllowedHeaders,
		}),
	)

	if cfg.HTTP.EnableAccessLog {
		router.Use(accesslog.AccessLog)
	}

	errTmpl := loadTemplate(
		cfg.HTTP.TemplateError,
		"error",
		cfg.HTTP.LiveReloadTemplate,
	)
	router.NotFound = func(w http.ResponseWriter, r *http.Request) {
		errHandler(
			errTmpl, w, r,
			errors.NotFoundf(`Here, take your URL back please (つ•᷄᎑•᷅)  %q.`, r.URL.Path),
		)
	}

	return router
}

func Start(cfg *configs.Config) func() {
	rms, ht := initServices(cfg)
	// Goroutine to broadcast room viewer counts to all rooms
	go func() {
		for {
			time.Sleep(time.Second * 5)

			roomViewers := map[string]int{}
			ht.sse.Clients.Range(func(c *sse.Client) {
				roomID, _ := roomIDUserNameFromSSEClientID(c.ID)
				roomViewers[roomID]++
			})

			for roomID, viewers := range roomViewers {
				ht.roomLiveViewers.Store(roomID, viewers)
				ht.roomBroadcast(roomID, &ssePayload{
					Type: SSEPTypeRoomViewers,
					Data: viewers,
				})
			}
			ht.api.Cleanup()
		}
	}()

	router := setupRouter(cfg, ht)
	go router.Start()

	return func() {
		if cfg.Rooms.BackupFilePath == "" {
			return
		}

		payload, err := rms.BackupRooms()
		if err != nil {
			// wrapping to get trace
			err = errors.Wrap(err)
			webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", err))
			return
		}

		f, err := os.OpenFile(cfg.Rooms.BackupFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModePerm)
		if err != nil {
			// wrapping to get trace
			err = errors.Wrap(err)
			webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", err))
			return
		}
		_, err = f.Write(payload)
		if err != nil {
			// wrapping to get trace
			err = errors.Wrap(err)
			webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", err))
			return
		}
	}
}
