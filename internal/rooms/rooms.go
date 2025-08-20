package rooms

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/users"
	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/naughtygopher/errors"
)

type Member struct {
	Token  uuid.UUID
	User   *users.User
	RoomID string
}

type Room struct {
	ID       string
	Name     string
	Capacity uint
	nameUsed map[string]struct{}
	Members  map[string]*Member
	// SecretHashKey is used to create a authn token per member
	SecretHashKey string
	msgs          *Messages
	Public        bool
	// Phantom if enabled would not retain messages on the server
	Phantom   bool
	CreatedAt time.Time
}

func (rm *Room) Sanitize() {
	rm.Name = strings.TrimSpace(rm.Name)
	if rm.Capacity == 0 {
		rm.Capacity = 250
	}

	rm.ID = slug.Make(rm.Name)

	if rm.Members == nil {
		rm.Members = make(map[string]*Member, rm.Capacity)
	}

	if rm.nameUsed == nil {
		rm.nameUsed = make(map[string]struct{}, rm.Capacity)
	}
}

func (rm *Room) Validate() error {
	if rm.ID == "" {
		return errors.Validation("room ID is required")
	}

	if rm.Name == "" {
		return errors.Validation("room name is required")
	}

	if rm.Capacity == 0 {
		return errors.Validation("room capacity must be greater than 0")
	}

	if len(rm.ID) > 512 || len(rm.Name) > 512 {
		return errors.Validation("room name or ID cannot be more than 512  bytes")
	}

	return nil
}

func (rm *Room) SanitizeValidate() error {
	rm.Sanitize()
	return rm.Validate()
}

func (rm *Room) AddMember(usr *users.User) (*Member, error) {
	_, exists := rm.nameUsed[usr.Name]
	if exists {
		return nil, errors.Duplicatef("member %q is already in the room %q", usr.Name, rm.ID)
	}

	member := &Member{
		Token:  uuid.New(),
		User:   usr,
		RoomID: rm.ID,
	}

	rm.Members[member.Token.String()] = member
	rm.nameUsed[usr.Name] = struct{}{}

	return member, nil
}

func (rm *Room) Messages() []messages.Message {
	return rm.msgs.All()
}

func (rm *Room) MembersList() []Member {
	list := make([]Member, 0, len(rm.Members))
	for key := range rm.Members {
		member := rm.Members[key]
		list = append(list, *member)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].User.Joined.Before(list[j].User.Joined)
	})
	return list
}

func NewRoom(id, name string, public, phantom bool, capacity uint) (*Room, error) {
	room := &Room{
		ID:        id,
		Name:      name,
		Capacity:  capacity,
		Members:   make(map[string]*Member, capacity),
		Public:    public,
		CreatedAt: time.Now(),
		Phantom:   phantom,
	}

	err := room.SanitizeValidate()
	if err != nil {
		return nil, err
	}

	room.msgs = NewMessages(room.Capacity*3, phantom)

	return room, nil
}

type Rooms struct {
	rooms    map[string]*Room
	capacity uint
	lock     *sync.RWMutex
}

func NewRooms(capacity uint) *Rooms {
	return &Rooms{
		rooms:    make(map[string]*Room),
		capacity: capacity,
		lock:     &sync.RWMutex{},
	}
}

func (rs *Rooms) Capacity() uint {
	return rs.capacity
}

func (rs *Rooms) Add(room *Room) (*Room, error) {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	if len(rs.rooms) >= int(rs.capacity) {
		return nil, errors.Errorf("rooms capacity exceeded")
	}

	if _, exists := rs.rooms[room.ID]; exists {
		return nil, errors.Duplicatef("room %q already exists", room.ID)
	}

	rs.rooms[room.ID] = room
	return room, nil
}

func (rs *Rooms) AddMember(room *Room, username string) (*Member, error) {
	user := &users.User{
		Name: username,
	}

	err := user.SanitizeValidate()
	if err != nil {
		return nil, err
	}

	member, err := room.AddMember(user)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (rs *Rooms) Join(roomID string, username string) (*Member, error) {
	room, err := rs.Room(roomID)
	if err != nil {
		return nil, err
	}

	return rs.AddMember(room, username)
}

func (rs *Rooms) AddAndJoin(roomID string, private, phantom bool, username string) (*Room, *Member, error) {
	room, err := NewRoom(roomID, roomID, private, phantom, 250)
	if err != nil {
		return nil, nil, err
	}

	room, err = rs.Add(room)
	if err != nil {
		return nil, nil, err
	}

	member, err := rs.AddMember(room, username)
	if err != nil {
		// it is ok to create a room without any user
		return room, nil, nil
	}

	return room, member, nil
}

func (rs *Rooms) List() ([]*Room, error) {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	rooms := make([]*Room, 0, len(rs.rooms))
	for _, room := range rs.rooms {
		if room.Public {
			rooms = append(rooms, room)
		}
	}

	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].CreatedAt.Before(rooms[j].CreatedAt)
	})

	return rooms, nil
}

func (rs *Rooms) Room(id string) (*Room, error) {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	room, exists := rs.rooms[id]
	if !exists {
		return nil, errors.NotFoundf("room %q not found", id)
	}

	return room, nil
}

func (rs *Rooms) ValidateMember(member *Member) (*Member, error) {
	room, err := rs.Room(member.RoomID)
	if err != nil {
		return nil, err
	}

	if _, exists := room.Members[member.Token.String()]; !exists {
		return nil, errors.Errorf("member %q not found in room %q", member.User.Name, room.ID)
	}

	return member, nil
}

func (rs *Rooms) NewMessage(roomID string, memberToken string, message string) (*messages.Message, error) {
	room, err := rs.Room(roomID)
	if err != nil {
		return nil, err
	}

	member, exists := room.Members[memberToken]
	if !exists {
		return nil, errors.Errorf("user %q is not a member of %q", member.User.Name, roomID)
	}

	msg, err := messages.New(member.User, room.ID, message)
	if err != nil {
		return nil, err
	}

	room.msgs.Add(msg)

	return msg, nil
}
