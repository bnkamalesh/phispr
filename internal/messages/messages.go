package messages

import (
	"strings"
	"time"

	"github.com/bnkamalesh/phispr/internal/users"
	"github.com/naughtygopher/errors"
)

type Message struct {
	ServerReceivedAt time.Time
	PublishedAt      time.Time

	RoomID  string
	Author  *users.User
	Content string
}

func (msg *Message) Sanitize() {
	// msg.Content = html.EscapeString(strings.TrimSpace(msg.Content))
	msg.Content = strings.TrimSpace(msg.Content)
	msg.ServerReceivedAt = msg.ServerReceivedAt.UTC()
	msg.PublishedAt = msg.PublishedAt.UTC()
}

func (msg *Message) Validate() error {
	if msg.Author == nil {
		return errors.Validation("message author is required")
	}

	if msg.RoomID == "" {
		return errors.Validation("message room ID is required")
	}

	if msg.Content == "" {
		return errors.Validation("message content is required")
	}

	if len(msg.Content) > 2048 {
		return errors.Validation("message is too big, max size is 2048 bytes")
	}

	return nil
}

func (msg *Message) SanitizeValidate() error {
	msg.Sanitize()
	return msg.Validate()
}

func New(author *users.User, roomID, content string) (*Message, error) {
	msg := &Message{
		ServerReceivedAt: time.Now(),
		RoomID:           roomID,
		Author:           author,
		Content:          content,
	}

	err := msg.SanitizeValidate()
	if err != nil {
		return nil, err
	}

	return msg, nil
}
