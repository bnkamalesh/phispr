package api

import (
	"github.com/bnkamalesh/chat/internal/messages"
	"github.com/bnkamalesh/chat/internal/rooms"
)

type chatrooms interface {
	AddAndJoin(roomID string, public bool, username string) (*rooms.Room, *rooms.Member, error)
	Join(roomID string, username string) (*rooms.Member, error)
	List() ([]*rooms.Room, error)
	Capacity() uint
	NewMessage(roomID string, memberToken string, message string) (*messages.Message, error)
	Room(id string) (*rooms.Room, error)
	ValidateMember(member *rooms.Member) (*rooms.Member, error)
}

type API struct {
	chatrooms
}

func NewAPI(rooms *rooms.Rooms) *API {
	return &API{
		chatrooms: rooms,
	}
}
