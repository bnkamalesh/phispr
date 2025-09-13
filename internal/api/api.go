package api

import (
	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/rooms"
)

type chatrooms interface {
	AddAndJoin(roomID string, private, phantom bool, username string, memberToken string) (*rooms.Room, *rooms.Member, error)
	Join(roomID string, username string, memberTokens string) (*rooms.Member, error)
	RemoveMember(roomID string, username string) (*rooms.Member, error)

	Public() ([]*rooms.Room, error)
	Unlisted(memberToken string) ([]*rooms.Room, error)
	Room(id string) (*rooms.Room, error)

	// Capacity is the number of members allowed in the room
	Capacity() uint
	// Total is the total number of rooms including unlisted ones
	Total() uint

	// NewMessage is used to post a new message to the room
	NewMessage(roomID string, memberToken string, message string) (*messages.Message, error)
	ValidateMember(member *rooms.Member) (*rooms.Member, error)
	MemberByUserID(roomID string, userID string) (*rooms.Member, error)

	// MaxMembers is the total number of users which can be accomodated across all the rooms
	MaxMembers() uint
	// Cleanup would delete idle rooms
	Cleanup()
}

type API struct {
	chatrooms
}

func NewAPI(rooms *rooms.Rooms) *API {
	return &API{
		chatrooms: rooms,
	}
}
