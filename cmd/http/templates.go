package http

import (
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/naughtygopher/errors"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/rooms"
)

// sillyAutoVersioning generates a checksum of all files in the given directory
// IMPORTANT: this may cause unnecessary performance issue, if it happens, must
// introduce https://github.com/naughtygopher/pocache
func sillyAutoVersioning(root string) (string, error) {
	checksum := sha256.New()
	err := filepath.WalkDir(
		root,
		func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(checksum, file); err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		return "", errors.Wrap(err, "directory walk failed")
	}

	return hex.EncodeToString(checksum.Sum(nil)), nil
}

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
	TotalRooms     uint          `json:"total_rooms"`
	LiveRooms      uint          `json:"live_rooms"`
	PublicRooms    uint          `json:"public_rooms"`
	Rooms          []*rooms.Room `json:"rooms"`
	UnlistedRooms  []*rooms.Room `json:"unlisted_rooms"`
	CurrentRelease string        `json:"current_release"`
}

type RoomPayload struct {
	RoomID           string             `json:"room_id"`
	RoomName         string             `json:"room_name"`
	Live             uint               `json:"live"`
	Capacity         uint               `json:"capacity"`
	MessageCapacity  uint               `json:"message_capacity"`
	Requestor        string             `json:"requestor"`
	Messages         []messages.Message `json:"messages"`
	Members          []rooms.Member     `json:"members"`
	Owner            string             `json:"owner"`
	Phantom          bool               `json:"phantom"`
	Public           bool               `json:"public"`
	BroadcastDelayMs uint               `json:"broadcast_delay_ms"`
	JSCookieName     string             `json:"js_cookie_name"`
}

type ErrorPayload struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
