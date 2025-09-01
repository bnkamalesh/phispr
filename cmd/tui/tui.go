package tui

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	phttp "github.com/bnkamalesh/phispr/cmd/http"
	"github.com/bnkamalesh/phispr/cmd/http/cli"
	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/rooms"
	"github.com/bnkamalesh/phispr/internal/users"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/naughtygopher/errors"
)

type Config struct {
	Host       string
	CookieFile string
	MemberFile string
}

type TUIMsg string
type TUI struct {
	cookieFile    *os.File
	memberFile    *os.File
	remote        *cli.CLI
	currentRoom   string
	currentMember *rooms.Member
}

func (t *TUI) Join(roomID, username string) (*rooms.Member, error) {
	t.currentRoom = roomID

	member, err := t.remote.JoinRoom(t.currentRoom, username)
	// dumb check for user already exists error {
	if err != nil && !strings.Contains(err.Error(), "[409]") {
		return nil, err
	}
	if member != nil {
		t.currentMember = member
		err = t.saveMember()
		if err != nil {
			return nil, err
		}
	}

	cjson, err := t.remote.CookiesJSON(t.currentRoom)
	if err != nil {
		return nil, err
	}

	newJS := map[string]any{}
	err = json.Unmarshal(cjson, &newJS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cookies")
	}

	currentCFile, err := io.ReadAll(t.cookieFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read current cookies")
	}

	if len(currentCFile) > 0 {
		currentJS := map[string]any{}
		err = json.Unmarshal(currentCFile, &currentJS)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal current cookies")
		}
		for key, value := range currentJS {
			_, found := newJS[key]
			if !found {
				newJS[key] = value
			}
		}
	}

	cjson, err = json.Marshal(newJS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal new cookies")
	}

	err = t.cookieFile.Truncate(0)
	if err != nil {
		return nil, err
	}

	_, err = t.cookieFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	_, err = t.cookieFile.Write(cjson)
	if err != nil {
		return nil, err
	}

	if member == nil {
		member = &rooms.Member{
			User: &users.User{
				Name: username,
			},
		}
	}

	return member, nil
}

func (t *TUI) saveMember() error {
	if t.memberFile == nil {
		return errors.New("member file is not initialized")
	}

	data, err := json.Marshal(t.currentMember)
	if err != nil {
		return errors.Wrap(err, "failed to marshal member data")
	}

	_, err = t.memberFile.Write(data)
	if err != nil {
		return errors.Wrap(err, "failed to write member data")
	}

	return nil
}

func (t *TUI) SendMessage(msg string) error {
	if msg == "" {
		return nil
	}

	return t.remote.SendMessage(t.currentRoom, msg)
}

func (t *TUI) Room() (*phttp.RoomPayload, error) {
	return t.remote.Room(t.currentRoom)
}

func (t *TUI) FormattedMsg(msg *messages.Message) string {
	header := fmt.Sprintf(
		"[%s] %s", msg.ServerReceivedAt.Format(time.DateTime),
		msg.Author.Name,
	)
	return fmt.Sprintf(
		"%s: %s",
		header,
		msg.Content,
	)
}

func (t *TUI) LoadMemberFromFile() error {
	if t.memberFile == nil {
		return nil
	}

	data, err := io.ReadAll(t.memberFile)
	if err != nil {
		return errors.Wrap(err, "failed to read member file")
	}

	var member rooms.Member
	err = json.Unmarshal(data, &member)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal member data")
	}

	t.currentMember = &member

	return nil
}

func New(cfg *Config) (*TUI, error) {
	c, err := cli.New(cfg.Host)
	if err != nil {
		return nil, err
	}

	cookieFile, err := os.OpenFile(cfg.CookieFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, err
	}

	js, err := io.ReadAll(cookieFile)
	if err != nil {
		return nil, err
	}

	err = c.LoadCookiesFromJSON(js)
	if err != nil {
		return nil, err
	}

	memberFile, err := os.OpenFile(cfg.MemberFile, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, err
	}

	t := &TUI{
		remote:     c,
		cookieFile: cookieFile,
		memberFile: memberFile,
	}

	return t, nil
}

func logExitOnErr(err error) {
	if err == nil {
		return
	}
	log.Fatal(err)
}

func Start() {
	configPath := flag.String("config", "~/Desktop", "-config='full/path/to/config.yaml'")
	flag.Parse()

	var k = koanf.New(".")
	k.Load(file.Provider(*configPath), yaml.Parser())

	t, err := New(&Config{
		Host:       k.MustString("host"),
		CookieFile: k.String("cookies_path"),
		MemberFile: k.String("member_path"),
	})
	logExitOnErr(err)

	roomID := k.String("room")
	username := k.String("username")

	if roomID != "" && username != "" {
		member, err := t.Join(roomID, username)
		logExitOnErr(err)
		t.currentMember = member
	}

	m := initialModel()
	m.currentRoom = roomID
	m.tui = t
	m.viewport.SetContent(fmt.Sprintf("===== Phispr [%s] ====", m.currentRoom))

	room, err := t.Room()
	logExitOnErr(err)
	for _, msg := range room.Messages {
		m.messages = append(m.messages, &msg)
	}

	p := tea.NewProgram(m)
	go func() {
		err := t.remote.Subscribe(roomID, func(sp *cli.SSEPayload) {
			switch sp.Type {
			case string(phttp.SSEPTypeRoomMessage):
				p.Send(sp.MessageReceived())
			case string(phttp.SSEPTypeRoomLeave):
				otherMember := m.memberStyle.Render(sp.MemberLeft().User.Name)
				p.Send(fmt.Sprintf("%s left the room", otherMember))
			case string(phttp.SSEPTypeRoomJoin):
				otherMember := m.memberStyle.Render(sp.MemberJoined().User.Name)
				p.Send(fmt.Sprintf("%s joined the room", otherMember))
			}
		})
		p.Send(err)
	}()

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
