package http

import (
	"html/template"
	"os"

	"github.com/bnkamalesh/chat/internal/messages"
	"github.com/bnkamalesh/chat/internal/rooms"
	"github.com/naughtygopher/errors"
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

func loadTemplate(path, name string) *template.Template {
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

type homePayload struct {
	TotalRooms  uint
	LiveRooms   uint
	PublicRooms uint
	Rooms       []*rooms.Room
}

type roomPayload struct {
	RoomID    string
	Live      uint
	Capacity  uint
	Requestor string
	Messages  []messages.Message
	Members   []rooms.Member
}

type errorPayload struct {
	Code    int
	Message string
}
