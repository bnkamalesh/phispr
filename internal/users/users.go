package users

import (
	"strings"
	"time"

	"github.com/naughtygopher/errors"
)

type User struct {
	Name        string
	Joined      time.Time
	LastPing    time.Time
	LastMessage time.Time
}

func (usr *User) Sanitize() {
	usr.Name = strings.TrimSpace(usr.Name)
	if usr.Joined.IsZero() {
		usr.Joined = time.Now()
	}
	if usr.LastPing.IsZero() {
		usr.LastPing = time.Now()
	}
	if usr.LastMessage.IsZero() {
		usr.LastMessage = time.Now()
	}
}

func (usr *User) Validate() error {
	if usr.Name == "" {
		return errors.Validation("user name is required")
	}

	if len(usr.Name) > 256 {
		return errors.Validation("user name is too long, max size is 256 bytes")
	}

	return nil
}

func (usr *User) SanitizeValidate() error {
	usr.Sanitize()
	return usr.Validate()
}
