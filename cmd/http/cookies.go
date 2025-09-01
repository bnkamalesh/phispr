package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/naughtygopher/errors"
)

const (
	cookieNameAnon     = "roomanonauth"
	cookieName         = "roomauth"
	cookieNameJS       = "roomauth_js"
	cookieNameMemberID = "member_id"
)

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

func setCookie(
	cookieName string,
	member *rooms.Member,
	roomPath string,
	httpOnly bool,
	cookieExpiry time.Time,
	w http.ResponseWriter,
) {
	jb, _ := json.Marshal(member)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    base64.StdEncoding.EncodeToString(jb),
		Path:     roomPath,
		HttpOnly: httpOnly,
		Expires:  cookieExpiry,
	})
}

func setMemberCookies(
	cookieName string,
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	cookieExpiry := time.Now().Add(240 * time.Hour)
	setCookie(
		cookieName, member, roomPath, true, cookieExpiry, w)
	setCookie(cookieNameJS, member, roomPath, false, cookieExpiry, w)
}

func removeMemberCookies(
	cookieName string,
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	cookieExpiry := time.Now().Add(time.Hour * -24)
	setCookie(cookieName, member, roomPath, true, cookieExpiry, w)
	setCookie(cookieNameJS, member, roomPath, false, cookieExpiry, w)
}

func setAnonCookie(
	member *rooms.Member,
	roomPath string,
	w http.ResponseWriter,
) {
	cookieExpiry := time.Now().Add(240 * time.Hour)
	setCookie(cookieNameAnon, member, roomPath, true, cookieExpiry, w)
}

func roomPath(roomID string) string {
	return "/rooms/" + url.QueryEscape(roomID)
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

func memberIDFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(cookieNameMemberID)
	if err != nil {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return ""
	}

	return string(decoded)
}

func setMemberIDCookie(memberID string, w http.ResponseWriter) {
	cookieExpiry := time.Now().Add(24 * 30 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieNameMemberID,
		Value:    base64.StdEncoding.EncodeToString([]byte(memberID)),
		Path:     "/",
		HttpOnly: true,
		Expires:  cookieExpiry,
	})
}
