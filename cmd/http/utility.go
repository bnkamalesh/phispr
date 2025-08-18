package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
)

func errorHandler(tmpl templateExecutor, w http.ResponseWriter, err error) {
	code, msg, _ := errors.HTTPStatusCodeMessage(err)
	if code >= http.StatusInternalServerError {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", err))
	}

	w.WriteHeader(code)
	if tmpl == nil {
		_, _ = w.Write([]byte(msg))
		return
	}

	terr := tmpl.Execute(w, &errorPayload{
		Code:    code,
		Message: msg,
	})
	if terr != nil {
		webgo.LOGHANDLER.Error(fmt.Sprintf("%+v", terr))
	}
}

func sseClientID(member *rooms.Member) string {
	if member == nil {
		return ""
	}

	roomID := base64.StdEncoding.EncodeToString([]byte(member.RoomID))
	userID := base64.StdEncoding.EncodeToString([]byte(member.User.Name))
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

func roomIDFromReq(r *http.Request) string {
	wctx := webgo.Context(r)
	roomID := wctx.URIParams["roomID"]
	roomID, _ = url.QueryUnescape(roomID)
	return roomID
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
		Name:     roomID,
		Value:    base64.StdEncoding.EncodeToString(jb),
		Path:     roomPath,
		HttpOnly: true,
		Expires:  cookieExpiry,
	})

	http.SetCookie(w, &http.Cookie{
		Name:    roomID + "_js",
		Value:   base64.StdEncoding.EncodeToString(jb),
		Path:    roomPath,
		Expires: cookieExpiry,
	})
}
