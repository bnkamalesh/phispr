package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"

	"github.com/bnkamalesh/phispr/internal/api"
)

type ssePType string

const (
	SSEPTypeRoomViewers ssePType = "room_viewers"
	SSEPTypeRoomMessage ssePType = "room_message"
	SSEPTypeRoomJoin    ssePType = "room_join"
	SSEPTypeRoomLeave   ssePType = "room_leave"
)

type ssePayload struct {
	Type ssePType
	Data any
}

var (
	lastModified = time.Now().Format(http.TimeFormat)
)

type templateExecutor interface {
	Execute(wr io.Writer, data any) error
}

type templateExecutorFunc func(wr io.Writer, data any) error

func (f templateExecutorFunc) Execute(wr io.Writer, data any) error {
	return f(wr, data)
}

type HTTP struct {
	sse               *sse.SSE
	api               *api.API
	manifestJSON      map[string]any
	manifestJSONBytes []byte
	staticRoot        string
	templateHome      templateExecutor
	templateRoom      templateExecutor
	templateErr       templateExecutor
	roomLiveViewers   sync.Map
	broadcastDelay    time.Duration
}

func (h *HTTP) Sanitize() {
	if h.broadcastDelay < time.Second {
		h.broadcastDelay = time.Second * 5
	}
}

func (h *HTTP) StaticAssetVersion(rw http.ResponseWriter, r *http.Request) {
	checksum, err := sillyAutoVersioning(h.staticRoot)
	if err != nil {
		errHandler(nil, rw, r, err)
		return
	}
	_, _ = rw.Write([]byte(checksum))
}

// StaticFilesHandler is used to serve static files
func (h *HTTP) StaticFilesHandler(rw http.ResponseWriter, r *http.Request) {
	wctx := webgo.Context(r)
	// '..' is replaced to prevent directory traversal which could go out of static directory
	path := strings.ReplaceAll(wctx.Params()["w"], "..", "-")
	path = strings.ReplaceAll(path, "~", "-")

	rw.Header().Set("Last-Modified", lastModified)
	rw.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=86400")
	if strings.Contains(path, "serviceworker.js") {
		rw.Header().Set("Service-Worker-Allowed", "/")
		rw.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFile(rw, r, fmt.Sprintf("%s/%s", h.staticRoot, path))
}

func (h *HTTP) PWAManifestHandler(rw http.ResponseWriter, r *http.Request) {

	rw.Header().Set("Last-Modified", lastModified)
	rw.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=86400")
	rw.Header().Set("Content-Type", "application/manifest+json")

	fromRoom := false
	referer := r.Header.Get("Referer")

	if strings.Contains(referer, "/rooms/") {
		fromRoom = true
	}

	if !fromRoom {
		_, _ = rw.Write(h.manifestJSONBytes)
		return
	}

	manifestCopy := map[string]any{}
	for k, v := range h.manifestJSON {
		manifestCopy[k] = v
	}

	manifestCopy["start_url"] = referer
	jb, err := json.Marshal(manifestCopy)
	if err != nil {
		errHandler(nil, rw, r, err)
		return
	}
	_, _ = rw.Write(jb)

}

func (h *HTTP) HomeHandler(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.api.Public()
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	memberToken := setupMemberToken(w, r)

	unlisted, err := h.api.Unlisted(memberToken)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	hp := &HomePayload{
		TotalRooms:          h.api.Capacity(),
		PublicRooms:         uint(len(rooms)),
		LiveRooms:           h.api.Total(),
		Rooms:               rooms,
		UnlistedRooms:       unlisted,
		CurrentRelease:      lastModified,
		StaticAssetChecksum: lastModified,
	}

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, hp, http.StatusOK)
	} else {
		terr := h.templateHome.Execute(w, hp)
		if terr != nil {
			webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
		}
	}
}

func setupMemberToken(w http.ResponseWriter, r *http.Request) string {
	memberToken := memberIDFromCookie(r)
	if memberToken == "" {
		memberToken = uuid.New().String()
	}
	setMemberIDCookie(memberToken, w)
	return memberToken
}

func errHandler(tmpl templateExecutor, w http.ResponseWriter, r *http.Request, err error) {
	code, msg, _ := errors.HTTPStatusCodeMessage(err)
	if code >= http.StatusInternalServerError {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", err))
	}

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendError(w, msg, code)
		return
	}

	w.WriteHeader(code)
	if tmpl == nil {
		_, _ = w.Write([]byte(msg))
		return
	}

	terr := tmpl.Execute(w, &ErrorPayload{
		Code:    code,
		Message: msg,
	})
	if terr != nil {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
	}
}

func (ht *HTTP) authOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		member, err := memberFromCookie(r)
		if err != nil || member == nil {
			webgo.SendError(w, err.Error(), http.StatusForbidden)
			return
		}

		room, err := ht.api.Room(member.RoomID)
		if err != nil {
			webgo.SendError(w, err.Error(), http.StatusForbidden)
			return
		}

		if !room.IsOwner(member) {
			webgo.SendError(
				w,
				"only the room owner is allowed to perform this action",
				http.StatusForbidden,
			)
			return
		}

		next(w, r)
	}
}
