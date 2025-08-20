package http

import (
	"context"
	"encoding/base64"
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
	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/bnkamalesh/phispr/internal/users"
)

type ssePType string

const (
	ssePTypeRoomViewers ssePType = "room_viewers"
	ssePTypeRoomMessage ssePType = "room_message"
	ssePTypeRoomJoin    ssePType = "room_join"
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
	sse             *sse.SSE
	api             *api.API
	templateHome    templateExecutor
	templateRoom    templateExecutor
	templateErr     templateExecutor
	roomLiveViewers sync.Map
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

	terr := h.templateHome.Execute(w, &homePayload{
		TotalRooms:  h.api.Capacity(),
		PublicRooms: uint(len(rooms)),
		LiveRooms:   uint(len(rooms)),
		Rooms:       rooms,
	})
	if terr != nil {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
	}
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

	val, _ := h.roomLiveViewers.Load(roomID)
	liveUsers, _ := val.(int)
	rp := &roomPayload{
		RoomID:    room.ID,
		RoomName:  room.Name,
		Capacity:  room.Capacity,
		Live:      uint(liveUsers),
		Requestor: requestor,
		Messages:  room.Messages(),
		Members:   room.MembersList(),
		Phantom:   room.Phantom,
		Public:    room.Public,
	}

	// pushRoompage(r, w)
	terr := h.templateRoom.Execute(w, rp)
	if terr != nil {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
	}
}

func (h *HTTP) CreateJoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	room, member, err := h.api.AddAndJoin(
		r.PostFormValue("name"),
		r.PostFormValue("unlisted") != "true",
		r.PostFormValue("phantom") == "true",
		r.PostFormValue("username"),
	)

	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	rp := roomPath(room.ID)
	setMemberCookies(cookieName, member, rp, w)

	http.Redirect(w, r, rp, http.StatusSeeOther)
}

func (h *HTTP) JoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	username := r.PostFormValue("username")

	member, err := h.api.Join(roomID, username)
	if err != nil {
		errorHandler(h.templateErr, w, err)
		return
	}

	rp := roomPath(roomID)
	setMemberCookies(cookieName, member, rp, w)
	http.Redirect(w, r, rp, http.StatusSeeOther)

	h.sse.Clients.Range(func(client *sse.Client) {
		roomID, _ := roomIDUserNameFromSSEClientID(client.ID)
		if roomID != member.RoomID {
			return
		}

		jb, _ := json.Marshal(ssePayload{
			Type: ssePTypeRoomJoin,
			Data: member,
		})
		client.Msg <- &sse.Message{
			Data: string(jb),
		}
	})
}

func (h *HTTP) NewMessage(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
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

		jb, _ := json.Marshal(ssePayload{
			Type: ssePTypeRoomMessage,
			Data: msg,
		})
		client.Msg <- &sse.Message{
			Data: string(jb),
		}
	})

}

func (h *HTTP) SSEHandler(w http.ResponseWriter, r *http.Request) {
	member, _ := h.memberOrAnonFromCookie(r)
	if member == nil {
		roomID := roomIDFromReq(r)
		// assume user is anonymous and has not joined the room yet
		member = &rooms.Member{
			RoomID: roomID,
			User:   &users.User{Name: "anon-" + uuid.New().String()},
		}
		setAnonCookie(member, roomPath(roomID), w)
	}

	clientID := sseClientID(member)

	cli := h.sse.Clients.Client(clientID)
	if cli != nil {
		h.sse.RemoveClient(r.Context(), clientID)
	}

	r.Header.Set(h.sse.ClientIDHeader, clientID)
	err := h.sse.Handler(w, r)
	if err != nil && !errors.Is(err, context.Canceled) {
		return
	}
}
func cookieMemberDetails(cookieName string, r *http.Request) (*rooms.Member, error) {
	roomID := roomIDFromReq(r)
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, errors.UnauthenticatedErrf(err, "you're not a member of the room %q", roomID)
	}
	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, errors.UnauthenticatedErrf(err, "could not verify your membership in the room %q", roomID)
	}

	member := &rooms.Member{}
	err = json.Unmarshal(decoded, member)
	if err != nil {
		return nil, errors.UnauthenticatedErrf(err, "could not verify your membership in the room %q", roomID)
	}
	return member, nil
}

func (ht *HTTP) memberFromCookie(r *http.Request) (*rooms.Member, error) {
	member, err := cookieMemberDetails(cookieName, r)
	if err != nil {
		return nil, err
	}

	member, err = ht.api.ValidateMember(member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (ht *HTTP) anonMemberFromCookie(r *http.Request) (*rooms.Member, error) {
	return cookieMemberDetails(cookieNameAnon, r)
}

func (ht *HTTP) memberOrAnonFromCookie(r *http.Request) (*rooms.Member, error) {
	member, err := ht.memberFromCookie(r)
	if err == nil {
		return member, nil
	}

	member, err = ht.anonMemberFromCookie(r)
	if err == nil {
		return member, nil
	}

	return nil, errors.New("no cookie")
}
