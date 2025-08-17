package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bnkamalesh/chat/internal/rooms"
	"github.com/naughtygopher/errors"
	"github.com/naughtygopher/webgo/v7"
)

func pushCSS(pusher http.Pusher, r *http.Request, path string) {
	cssOpts := &http.PushOptions{
		Header: http.Header{
			"Accept-Encoding": r.Header["Accept-Encoding"],
			"Content-Type":    []string{"text/css; charset=UTF-8"},
		},
	}
	err := pusher.Push(path, cssOpts)
	if err != nil {
		webgo.LOGHANDLER.Error(err)
	}
}

func pushJS(pusher http.Pusher, r *http.Request, path string) {
	cssOpts := &http.PushOptions{
		Header: http.Header{
			"Accept-Encoding": r.Header["Accept-Encoding"],
			"Content-Type":    []string{"application/javascript"},
		},
	}
	err := pusher.Push(path, cssOpts)
	if err != nil {
		webgo.LOGHANDLER.Error(err)
	}
}

func pushCommon(r *http.Request, w http.ResponseWriter) http.Pusher {
	pusher, ok := w.(http.Pusher)
	if !ok {
		return nil
	}

	cp, _ := r.Cookie("pusher")
	if cp != nil {
		return pusher
	}

	cookie := &http.Cookie{
		Name:   "pusher",
		Value:  "css,js",
		MaxAge: 300,
	}

	http.SetCookie(w, cookie)
	pushCSS(pusher, r, "/static/css/main.css")
	pushCSS(pusher, r, "/static/css/normalize.css")
	pushJS(pusher, r, "/static/js/sse.js")
	pushJS(pusher, r, "/static/js/common.js")
	return pusher
}

func errorHandler(tmpl *template.Template, w http.ResponseWriter, err error) {
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

func pushHomepage(r *http.Request, w http.ResponseWriter) {
	pusher := pushCommon(r, w)
	if pusher != nil {
		pushJS(pusher, r, "/static/js/room.js")
	}
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

func pushRoompage(r *http.Request, w http.ResponseWriter) {
	pusher := pushCommon(r, w)
	if pusher != nil {
		pushJS(pusher, r, "/static/js/main.js")
	}
}
