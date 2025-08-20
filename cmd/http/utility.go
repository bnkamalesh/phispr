package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"

	"github.com/bnkamalesh/phispr/internal/rooms"
)

const (
	cookieNameAnon = "roomanonauth"
	cookieName     = "roomauth"
	cookieNameJS   = "roomauth-js"
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
func setCookie(
	cookieName string,
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	jb, _ := json.Marshal(member)
	cookieExpiry := time.Now().Add(240 * time.Hour)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    base64.StdEncoding.EncodeToString(jb),
		Path:     roomPath,
		HttpOnly: true,
		Expires:  cookieExpiry,
	})
}
func setMemberCookies(
	cookieName string,
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	setCookie(cookieName, member, roomPath, w)
	setCookie(cookieNameJS, member, roomPath, w)
}

func setAnonCookie(
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	setCookie(cookieNameAnon, member, roomPath, w)
}

func roomPath(roomID string) string {
	return "/rooms/" + url.QueryEscape(roomID)
}
