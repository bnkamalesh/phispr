package http

import (
	"html/template"
	"io"
	"os"

	"github.com/naughtygopher/errors"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/rooms"
)

func readFile(path string) (string, error) {
	fs, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return "", errors.Wrap(err, "failed to load %q", path)
	}
	defer fs.Close()

	info, err := fs.Stat()
	if err != nil {
		return "", errors.Wrap(err, "failed reading %q info", path)
	}

	out := make([]byte, info.Size())
	_, err = fs.Read(out)
	if err != nil {
		return "", errors.Wrap(err, "failed reading %q", path)
	}

	return string(out), nil
}

func loadRawTemplate(path, name string) templateExecutor {
	out, err := readFile(path)
	if err != nil {
		panic(err)
	}

	tmpl, err := template.New(name).Parse(out)
	if err != nil {
		panic(errors.Wrapf(err, "failed to parse template(%s:%s)", path, name))
	}
	return tmpl
}

func loadTemplate(path, name string, livereload bool) templateExecutor {
	if !livereload {
		return loadRawTemplate(path, name)
	}

	return templateExecutorFunc(func(wr io.Writer, data any) error {
		return loadRawTemplate(path, name).Execute(wr, data)
	})
}

type HomePayload struct {
	TotalRooms    uint          `json:"total_rooms,omitempty"`
	LiveRooms     uint          `json:"live_rooms,omitempty"`
	PublicRooms   uint          `json:"public_rooms,omitempty"`
	Rooms         []*rooms.Room `json:"rooms,omitempty"`
	UnlistedRooms []*rooms.Room `json:"unlisted_rooms,omitempty"`
}

type RoomPayload struct {
	RoomID           string             `json:"room_id,omitempty"`
	RoomName         string             `json:"room_name,omitempty"`
	Live             uint               `json:"live,omitempty"`
	Capacity         uint               `json:"capacity,omitempty"`
	MessageCapacity  uint               `json:"message_capacity,omitempty"`
	Requestor        string             `json:"requestor,omitempty"`
	Messages         []messages.Message `json:"messages,omitempty"`
	Members          []rooms.Member     `json:"members,omitempty"`
	Owner            string             `json:"owner,omitempty"`
	Phantom          bool               `json:"phantom,omitempty"`
	Public           bool               `json:"public,omitempty"`
	BroadcastDelayMs uint               `json:"broadcast_delay_ms,omitempty"`
	JSCookieName     string             `json:"js_cookie_name,omitempty"`
}

type ErrorPayload struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
