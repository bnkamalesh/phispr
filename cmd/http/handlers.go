package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bnkamalesh/chat/internal/api"
	"github.com/bnkamalesh/chat/internal/rooms"
	"github.com/bnkamalesh/chat/internal/users"
	"github.com/google/uuid"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"
)

type HTTP struct {
	sse          *sse.SSE
	api          *api.API
	templateHome *template.Template
	templateRoom *template.Template
	templateErr  *template.Template
}

// StaticFilesHandler is used to serve static files
func (h *HTTP) StaticFilesHandler(rw http.ResponseWriter, r *http.Request) {
	wctx := webgo.Context(r)
	// '..' is replaced to prevent directory traversal which could go out of static directory
	path := strings.ReplaceAll(wctx.Params()["w"], "..", "-")
	path = strings.ReplaceAll(path, "~", "-")

	rw.Header().Set("Last-Modified", lastModified)
	http.ServeFile(rw, r, fmt.Sprintf("./cmd/http/static/%s", path))
}

func (h *HTTP) HomeHandler(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.api.List()
	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	pushHomepage(r, w)
	h.templateHome.Execute(w, &homePayload{
		TotalRooms:  h.api.Capacity(),
		PublicRooms: uint(len(rooms)),
		LiveRooms:   uint(len(rooms)),
		Rooms:       rooms,
	})
}

func (h *HTTP) RoomHandler(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	room, err := h.api.Room(roomID)
	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	requestor := ""
	member, err := h.memberFromCookie(r)
	if err == nil {
		requestor = member.User.Name
	}

	rp := &roomPayload{
		RoomID:    room.ID,
		Capacity:  room.Capacity,
		Live:      uint(len(room.Members)),
		Requestor: requestor,
		Messages:  room.Messages(),
	}

	pushRoompage(r, w)
	h.templateRoom.Execute(w, rp)
}

func setMemberCookies(
	member *rooms.Member,
	roomID string,
	roomPath string,
	w http.ResponseWriter,
) {
	jb, _ := json.Marshal(member)
	cookieExpiry := time.Now().Add(240 * time.Hour) // Set cookie expiry to 24 hours
	http.SetCookie(w, &http.Cookie{
		Name:     base64.StdEncoding.EncodeToString([]byte(roomID)),
		Value:    base64.StdEncoding.EncodeToString(jb),
		Path:     roomPath,
		HttpOnly: true,
		Expires:  cookieExpiry,
	})

	http.SetCookie(w, &http.Cookie{
		Name:    base64.StdEncoding.EncodeToString([]byte(roomID + "_js")),
		Value:   base64.StdEncoding.EncodeToString(jb),
		Path:    roomPath,
		Expires: cookieExpiry,
	})
}

func (h *HTTP) CreateJoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	room, member, err := h.api.AddAndJoin(
		r.PostFormValue("name"),
		true,
		r.PostFormValue("username"),
	)
	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	roomPath := "/rooms/" + room.ID
	setMemberCookies(member, room.ID, roomPath, w)

	http.Redirect(w, r, roomPath, http.StatusSeeOther)
}

func (h *HTTP) JoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	wctx := webgo.Context(r)
	roomID := wctx.URIParams["roomID"]
	username := r.PostFormValue("username")

	member, err := h.api.Join(roomID, username)
	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	roomPath := "/rooms/" + roomID
	setMemberCookies(member, roomID, roomPath, w)
	http.Redirect(w, r, roomPath, http.StatusSeeOther)
}

func (h *HTTP) NewMessage(w http.ResponseWriter, r *http.Request) {
	wctx := webgo.Context(r)
	roomID := wctx.URIParams["roomID"]
	member, err := h.memberFromCookie(r)
	if err != nil {
		errorHandler(nil, w, err)
		return
	}

	if roomID != member.RoomID {
		errorHandler(nil, w, errors.Unauthenticatedf("%q is not a member of the room %q", member.User.Name, roomID))
		return
	}

	message := r.PostFormValue("message")
	msg, err := h.api.NewMessage(member.RoomID, member.Token.String(), message)
	if err != nil {
		errorHandler(nil, w, err)
		return
	}

	webgo.SendResponse(w, msg, http.StatusOK)
	h.sse.Clients.Range(func(client *sse.Client) {
		roomID, _ := roomIDUserNameFromSSEClientID(client.ID)
		if roomID != member.RoomID {
			return
		}

		jb, _ := json.Marshal(msg)
		client.Msg <- &sse.Message{
			Data: string(jb),
		}
	})
}

func pushHomepage(r *http.Request, w http.ResponseWriter) {
	pusher := pushCommon(r, w)
	if pusher != nil {
		pushJS(pusher, r, "/static/js/room.js")
	}
}

func pushRoompage(r *http.Request, w http.ResponseWriter) {
	pusher := pushCommon(r, w)
	if pusher != nil {
		pushJS(pusher, r, "/static/js/main.js")
	}
}

func (h *HTTP) SSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		member, _ := h.memberFromCookie(r)
		if member == nil {
			roomID := roomIDFromReq(r)
			// assume user is anonymous and has not joined the room yet
			member = &rooms.Member{
				RoomID: roomID,
				User:   &users.User{Name: "anon-" + uuid.New().String()},
			}
		}

		clientID := sseClientID(member)

		cli := h.sse.Clients.Client(clientID)
		if cli != nil {
			h.sse.RemoveClient(r.Context(), clientID)
		}

		r.Header.Set(h.sse.ClientIDHeader, clientID)
		err := h.sse.Handler(w, r)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Println("errorLogger:", err.Error())
			return
		}
	}
}
