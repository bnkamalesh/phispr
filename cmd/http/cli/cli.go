package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	phttp "github.com/bnkamalesh/phispr/cmd/http"
	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/naughtygopher/errors"
)

type CLI struct {
	http    *http.Client
	Address string
	OnSSE   func()
}

type SSEPayload struct {
	Type string
	Data json.RawMessage
}

func (ssep *SSEPayload) MemberJoined() *rooms.Member {
	if ssep.Type != string(phttp.SSEPTypeRoomJoin) {
		return nil
	}
	mem := new(rooms.Member)
	_ = json.Unmarshal(ssep.Data, mem)
	return mem
}

func (ssep *SSEPayload) MemberLeft() *rooms.Member {
	if ssep.Type != string(phttp.SSEPTypeRoomLeave) {
		return nil
	}
	mem := new(rooms.Member)
	_ = json.Unmarshal(ssep.Data, mem)
	return mem
}

func (ssep *SSEPayload) MessageReceived() *messages.Message {
	if ssep.Type != string(phttp.SSEPTypeRoomMessage) {
		return nil
	}
	msg := new(messages.Message)
	_ = json.Unmarshal(ssep.Data, msg)
	return msg
}

func (ssep *SSEPayload) ViewCountChanged() uint {
	if ssep.Type != string(phttp.SSEPTypeRoomViewers) {
		return 0
	}
	vc := uint(0)
	_ = json.Unmarshal(ssep.Data, &vc)
	return vc
}

func (cl *CLI) JoinRoom(roomID, userID string) (*rooms.Member, error) {

	jb, err := json.Marshal(map[string]string{
		"username": userID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed encoding input")
	}

	u := fmt.Sprintf("%s/rooms/%s", cl.Address, roomID)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(jb))
	if err != nil {
		return nil, errors.Wrap(err, "failed creating join request")
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed joining room %q", roomID)
	}

	if resp.StatusCode != http.StatusOK {
		str, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf(
			"[%d] %s",
			resp.StatusCode,
			string(str),
		)
	}

	jp := struct {
		Data *rooms.Member `json:"data"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&jp)
	if err != nil {
		return nil, errors.Wrap(err, "failed decoding response")
	}

	return jp.Data, nil
}

func (cl *CLI) LeaveRoom(roomID, userID string) (*rooms.Member, error) {

	jb, err := json.Marshal(map[string]string{
		"username": userID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed encoding input")
	}

	u := fmt.Sprintf("%s/rooms/%s/leave", cl.Address, roomID)
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(jb))
	if err != nil {
		return nil, errors.Wrap(err, "failed creating leave request")
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed joining room %q", roomID)
	}

	if resp.StatusCode != http.StatusOK {
		str, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf(
			"[%d] %s",
			resp.StatusCode,
			string(str),
		)
	}

	jp := struct {
		Data *rooms.Member `json:"data"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&jp)
	if err != nil {
		return nil, errors.Wrap(err, "failed decoding response")
	}

	return jp.Data, nil
}

func (cl *CLI) CookiesJSON(roomID string) ([]byte, error) {
	u, err := url.Parse(fmt.Sprintf("%s/rooms/%s", cl.Address, roomID))
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing URL:%s", cl.Address)
	}

	uri := u.String()

	payload := map[string][]http.Cookie{}
	jarCookies := cl.http.Jar.Cookies(u)
	for _, cookie := range jarCookies {
		list, ok := payload[uri]
		if !ok {
			list = make([]http.Cookie, 0, len(jarCookies))
		}

		cookie.Path = u.Path
		// cookie.HttpOnly = true

		if cookie.Expires.IsZero() {
			cookie.Expires = time.Now().Add(time.Hour * 240)
		}
		list = append(list, *cookie)

		payload[uri] = list
	}

	jb, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling cookies")
	}

	return jb, nil
}

func (cl *CLI) LoadCookiesFromJSON(js []byte) error {
	if len(js) == 0 {
		return nil
	}

	payload := map[string][]*http.Cookie{}
	err := json.Unmarshal(js, &payload)
	if err != nil {
		return errors.Wrap(err, "failed unmarshaling cookies")
	}

	for uri, cookies := range payload {
		u, err := url.Parse(uri)
		if err != nil {
			return errors.Wrapf(err, "failed parsing rawQ: %s", uri)
		}
		cl.http.Jar.SetCookies(u, cookies)
	}

	return nil
}

func (cl *CLI) SendMessage(roomID string, message string) error {
	jb, err := json.Marshal(map[string]string{
		"message": message,
	})
	if err != nil {
		return errors.Wrap(err, "failed marshaling JSON")
	}

	u := fmt.Sprintf("%s/rooms/%s/messages", cl.Address, roomID)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(jb))
	if err != nil {
		return errors.Wrap(err, "failed creating new message request")
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.http.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed joining room %q", roomID)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return errors.Errorf("[%d] %s", resp.StatusCode, string(b))
	}

	return nil
}

func (cl *CLI) Room(roomID string) (*phttp.RoomPayload, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/rooms/%s", cl.Address, roomID))
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating join request")
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.http.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting room details")
	}

	payload := struct {
		Data *phttp.RoomPayload `json:"data,omitempty"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	return payload.Data, nil
}

func (cl *CLI) subscribe(
	roomID string,
	req *http.Request,
	onSSEUpdate func(*SSEPayload),
) error {
	resp, err := cl.http.Do(req)
	if err != nil {
		return errors.Wrapf(
			err,
			"disconnected, cannot receive messages from %q",
			roomID,
		)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return errors.Wrapf(
				err,
				"disconnected, cannot receive messages from %q",
				roomID,
			)
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		line = strings.TrimPrefix(line, "data:")
		s := new(SSEPayload)
		err = json.Unmarshal([]byte(line), s)
		if err != nil {
			return errors.Wrapf(err, "subscription stopped: %s", line)
		}

		if onSSEUpdate != nil {
			onSSEUpdate(s)
		}
	}
}

func (cl *CLI) Subscribe(roomID string, onSSEUpdate func(*SSEPayload)) error {
	u, _ := url.Parse(fmt.Sprintf("%s/rooms/%s/messages", cl.Address, roomID))
	retries := 3
	errList := make([]error, 0, retries)
	for retries > 0 {
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return errors.Wrap(err, "failed preparing subscription request")
		}
		err = cl.subscribe(roomID, req, onSSEUpdate)
		errList = append(errList, err)
		retries--
	}

	return errors.Join(errList...)
}

func New(address string) (*CLI, error) {
	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed initializing cookiejar")
	}

	hcli := http.Client{
		Jar: cj,
	}

	return &CLI{
		Address: address,
		http:    &hcli,
	}, nil
}
