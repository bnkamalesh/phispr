package users

import (
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/naughtygopher/errors"
)

type User struct {
	Name        string
	ID          string
	Joined      time.Time
	LastMessage time.Time
	LastPing    time.Time
}

func (usr *User) Sanitize() {
	usr.Name = strings.TrimSpace(usr.Name)
	usr.ID = slug.Make(usr.Name)

	now := time.Now()
	if usr.Joined.IsZero() {
		usr.Joined = now
	}

	if usr.LastMessage.IsZero() {
		usr.LastMessage = now
	}

	if usr.LastPing.IsZero() {
		usr.LastPing = now
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
