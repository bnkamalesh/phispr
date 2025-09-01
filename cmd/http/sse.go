package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/bnkamalesh/phispr/internal/users"
	"github.com/google/uuid"
	"github.com/naughtygopher/errors"
)

func (h *HTTP) SSEHandler(w http.ResponseWriter, r *http.Request) {
	maxConns := h.api.MaxMembers()
	if h.sse.ActiveClients()+1 > int(maxConns) {
		errHandler(
			h.templateErr, w, r,
			errors.MaximumAttemptsf(
				"maximum number of connections reached (%d)", maxConns,
			),
		)
		return
	}

	memberToken := memberIDFromCookie(r)
	if memberToken == "" {
		memberToken = uuid.New().String()
		setMemberIDCookie(memberToken, w)
	}

	member, _ := h.memberOrAnonFromCookie(r)
	if member == nil {
		roomID := roomIDFromReq(r)
		// assume user is anonymous and has not joined the room yet
		member = &rooms.Member{
			RoomID: roomID,
			User:   &users.User{Name: "anon-" + memberToken},
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

func sseClientID(member *rooms.Member) string {
	if member == nil {
		return ""
	}

	roomID := base64.StdEncoding.EncodeToString([]byte(member.RoomID))
	userID := base64.StdEncoding.EncodeToString([]byte(member.User.ID))
	merged := fmt.Sprintf("%s/%s", roomID, userID)

	return base64.StdEncoding.EncodeToString([]byte(merged))
}

func roomIDUserNameFromSSEClientID(sseID string) (roomID string, userID string) {
	decoded, err := base64.StdEncoding.DecodeString(sseID)
	if err != nil {
		return "", ""
	}

	parts := strings.SplitN(string(decoded), "/", 2)
	if len(parts) != 2 {
		return "", ""
	}

	roomid, _ := base64.StdEncoding.DecodeString(parts[0])
	userid, _ := base64.StdEncoding.DecodeString(parts[1])

	return string(roomid), string(userid)
}
