package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
	"github.com/naughtygopher/webgo/v7/extensions/sse"
)

func roomIDFromReq(r *http.Request) string {
	wctx := webgo.Context(r)
	roomID := wctx.URIParams["roomID"]
	roomID, _ = url.QueryUnescape(roomID)
	return roomID
}

func (h *HTTP) roomBroadcast(toRoomID string, ssep *ssePayload) {
	jb, _ := json.Marshal(ssep)
	h.sse.Clients.Range(func(client *sse.Client) {
		roomID, userID := roomIDUserNameFromSSEClientID(client.ID)
		if roomID != toRoomID {
			return
		}

		member, _ := h.api.MemberByUserID(roomID, userID)
		if member != nil {
			member.User.LastPing = time.Now()
		}

		client.Msg <- &sse.Message{
			Data: string(jb),
		}
	})
}

func (h *HTTP) RoomHandler(w http.ResponseWriter, r *http.Request) {
	_ = setupMemberToken(w, r)

	roomID := roomIDFromReq(r)
	room, err := h.api.Room(roomID)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	requestor := ""
	member, err := h.memberFromCookie(r)
	if err == nil {
		requestor = member.User.ID
	}

	owner := ""
	if room.Owner != nil {
		owner = room.Owner.User.ID
	}

	val, _ := h.roomLiveViewers.Load(roomID)
	liveUsers, _ := val.(int)
	rp := &RoomPayload{
		RoomID:              room.ID,
		RoomName:            room.Name,
		Capacity:            room.Capacity,
		Live:                uint(liveUsers),
		MessageCapacity:     room.MessageRetentionCapacity(),
		Requestor:           requestor,
		Messages:            room.Messages(),
		Members:             room.MembersList(),
		Owner:               owner,
		Phantom:             room.Phantom,
		Public:              room.Listed,
		BroadcastDelayMs:    uint(h.broadcastDelay.Milliseconds()),
		JSCookieName:        cookieNameJS,
		StaticAssetChecksum: lastModified,
	}

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, rp, http.StatusOK)
	} else {
		terr := h.templateRoom.Execute(w, rp)
		if terr != nil {
			webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
		}
	}
}

func createJoinPayload(r *http.Request) (*rooms.Room, string, error) {
	defer r.Body.Close()

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		payload := struct {
			Name     string `json:"name,omitempty"`
			Unlisted bool   `json:"unlisted,omitempty"`
			Phantom  bool   `json:"phantom,omitempty"`
			Username string `json:"username,omitempty"`
		}{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			return nil, "", errors.InputBodyErr(err, "failed decoding input")
		}

		return &rooms.Room{
			Name:    payload.Name,
			Listed:  !payload.Unlisted,
			Phantom: payload.Phantom,
		}, payload.Username, nil
	}

	return &rooms.Room{
		Name:    r.PostFormValue("name"),
		Listed:  r.PostFormValue("unlisted") != "true",
		Phantom: r.PostFormValue("phantom") == "true",
	}, r.PostFormValue("username"), nil
}

func (h *HTTP) CreateJoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	cjp, username, err := createJoinPayload(r)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	memberToken := setupMemberToken(w, r)

	room, member, err := h.api.AddAndJoin(
		cjp.Name,
		cjp.Listed,
		cjp.Phantom,
		username,
		memberToken,
	)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	rp := roomPath(room.ID)
	setMemberCookies(cookieRoomAuth, member, rp, w)

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, rp, http.StatusOK)
	} else {
		http.Redirect(w, r, rp, http.StatusSeeOther)
	}
}

func joinPayload(r *http.Request) (string, error) {
	defer r.Body.Close()

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		payload := struct {
			Username string `json:"username,omitempty"`
		}{}

		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			return "", errors.InputBodyErr(err, "failed decoding input")
		}

		return payload.Username, nil
	}

	return r.PostFormValue("username"), nil
}

func (h *HTTP) JoinRoomHandler(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	username, err := joinPayload(r)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	memberToken := setupMemberToken(w, r)

	member, err := h.api.Join(roomID, username, memberToken)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	rp := roomPath(roomID)
	setMemberCookies(cookieRoomAuth, member, rp, w)

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, member, http.StatusOK)
	} else {
		http.Redirect(w, r, rp, http.StatusSeeOther)
	}

	room, _ := h.api.Room(roomID)
	payload := struct {
		rooms.Member
		TotalMembers uint
	}{}
	payload.Member = *member
	payload.TotalMembers = uint(len(room.Members))

	h.roomBroadcast(member.RoomID, &ssePayload{
		Type: SSEPTypeRoomJoin,
		Data: payload,
	})
}

func (h *HTTP) LeaveRoomHandler(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	username, err := joinPayload(r)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	member, err := h.api.RemoveMember(roomID, username)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	rp := roomPath(roomID)
	removeMemberCookies(cookieRoomAuth, member, rp, w)

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, member, http.StatusOK)
	} else {
		http.Redirect(w, r, rp, http.StatusSeeOther)
	}

	room, _ := h.api.Room(roomID)
	payload := struct {
		rooms.Member
		TotalMembers uint
	}{}
	payload.Member = *member
	payload.TotalMembers = uint(len(room.Members))

	h.roomBroadcast(roomID, &ssePayload{
		Type: SSEPTypeRoomLeave,
		Data: payload,
	})
}

func (h *HTTP) BootUserHandler(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	wctx := webgo.Context(r)
	username := wctx.URIParams["userID"]

	member, err := h.api.RemoveMember(roomID, username)
	if err != nil {
		errHandler(h.templateErr, w, r, err)
		return
	}

	rp := roomPath(roomID)

	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		webgo.SendResponse(w, member, http.StatusOK)
	} else {
		http.Redirect(w, r, rp, http.StatusSeeOther)
	}

	room, _ := h.api.Room(roomID)
	payload := struct {
		rooms.Member
		TotalMembers uint
	}{}
	payload.Member = *member
	payload.TotalMembers = uint(len(room.Members))

	h.roomBroadcast(roomID, &ssePayload{
		Type: SSEPTypeRoomLeave,
		Data: payload,
	})
}

func newMessagePayload(r *http.Request) (string, error) {
	if strings.Contains(r.Header.Get("Content-type"), "application/json") {
		payload := struct {
			Message string `json:"message,omitempty"`
		}{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			return "", errors.Wrap(err, "failed decoding input body")
		}

		return payload.Message, nil
	}

	return r.PostFormValue("message"), nil
}

func (h *HTTP) NewMessage(w http.ResponseWriter, r *http.Request) {
	roomID := roomIDFromReq(r)
	member, err := h.memberFromCookie(r)
	if err != nil {
		errHandler(nil, w, r, err)
		return
	}

	if roomID != member.RoomID {
		errHandler(
			nil, w, r,
			errors.Unauthenticatedf("%q is not a member of the room %q", member.User.Name, roomID),
		)
		return
	}

	message, err := newMessagePayload(r)
	if err != nil {
		errHandler(nil, w, r, err)
		return
	}

	msg, err := h.api.NewMessage(member.RoomID, member.Token, message)
	if err != nil {
		errHandler(nil, w, r, err)
		return
	}

	webgo.SendResponse(w, msg, http.StatusOK)

	h.roomBroadcast(member.RoomID, &ssePayload{
		Type: SSEPTypeRoomMessage,
		Data: msg,
	})
}
