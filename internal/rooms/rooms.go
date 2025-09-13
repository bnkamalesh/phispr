package rooms

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/users"
	"github.com/naughtygopher/errors"
)

const roomCapacity uint = 250

type Member struct {
	Token  string
	User   *users.User
	RoomID string
}

type Rooms struct {
	rooms map[string]*Room
	// capacity is the number of rooms which be hosted
	capacity uint
	// memberCapacity is the maximum number of members per room
	memberCapacity uint
	idleRoomExpiry time.Duration
	lock           *sync.RWMutex
}

func (rs *Rooms) Capacity() uint {
	return rs.capacity
}

func (rs *Rooms) Total() uint {
	return uint(len(rs.rooms))
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

func (rs *Rooms) AddMember(room *Room, username string, memberToken string) (*Member, error) {
	user := &users.User{
		Name: username,
	}

	err := user.SanitizeValidate()
	if err != nil {
		return nil, err
	}

	member, err := room.AddMember(user, memberToken)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (rs *Rooms) Join(roomID string, username string, memberToken string) (*Member, error) {
	room, err := rs.Room(roomID)
	if err != nil {
		return nil, err
	}

	return rs.AddMember(room, username, memberToken)
}

func (rs *Rooms) RemoveMember(roomID string, username string) (*Member, error) {
	room, err := rs.Room(roomID)
	if err != nil {
		return nil, err
	}

	return room.RemoveMemberAutoAssignOwner(username)
}

func (rs *Rooms) AddAndJoin(roomID string, private, phantom bool, username, memberToken string) (*Room, *Member, error) {
	room, err := NewRoom(
		roomID,
		roomID,
		private,
		phantom,
		rs.memberCapacity,
		rs.idleRoomExpiry,
	)
	if err != nil {
		return nil, nil, err
	}

	room, err = rs.Add(room)
	if err != nil {
		return nil, nil, err
	}

	member, err := rs.AddMember(room, username, memberToken)
	if err != nil {
		// it is ok to create a room without any user
		return room, nil, nil
	}

	return room, member, nil
}

func (rs *Rooms) Public() ([]*Room, error) {
	rs.lock.RLock()
	defer rs.lock.RUnlock()

	rooms := make([]*Room, 0, len(rs.rooms))
	for _, room := range rs.rooms {
		if room.Listed {
			rooms = append(rooms, room)
		}
	}

	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].CreatedAt.Before(rooms[j].CreatedAt)
	})

	return rooms, nil
}

func (rs *Rooms) Unlisted(memberToken string) ([]*Room, error) {
	if memberToken == "" {
		return nil, nil
	}

	rs.lock.RLock()
	defer rs.lock.RUnlock()

	rooms := make([]*Room, 0, len(rs.rooms))
	for _, room := range rs.rooms {
		if !room.Listed && room.HasMember(memberToken) {
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

	if _, exists := room.Members[member.Token]; !exists {
		return nil, errors.Errorf("member %q not found in room %q", member.User.Name, room.ID)
	}

	return member, nil
}

// BackupRooms returns a JSON of all the rooms
func (rs *Rooms) BackupRooms() ([]byte, error) {
	payload, err := json.Marshal(rs.rooms)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating rooms backup")
	}
	return payload, nil
}

func (rs *Rooms) LoadRoomsBackup(payload []byte) error {
	r := make(map[string]*Room)
	err := json.Unmarshal(payload, &r)
	if err != nil {
		return errors.Wrap(err, "failed loading rooms from backup")
	}
	for id := range r {
		room := r[id]
		if room.Members == nil {
			room.Members = make(map[string]*Member)
		}
		room.initMessages()
	}
	rs.rooms = r
	return nil
}

func (rs *Rooms) Cleanup() {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	for id, room := range rs.rooms {
		if room.Cleanup() {
			delete(rs.rooms, id)
		}
	}
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

	if !room.Phantom {
		room.msgs.Add(msg)
	}

	now := time.Now()
	room.LastActivityAt = now
	member.User.LastMessage = now

	return msg, nil
}

func (rs *Rooms) MaxMembers() uint {
	return rs.capacity * roomCapacity
}

func (rs *Rooms) MemberByUserID(roomID string, userID string) (*Member, error) {
	room, err := rs.Room(roomID)
	if err != nil {
		return nil, err
	}
	return room.MemberByUserID(userID)
}

func NewRooms(capacity, roomCapacity uint, idleRoomExpiry time.Duration) *Rooms {
	return &Rooms{
		rooms:          make(map[string]*Room),
		capacity:       capacity,
		memberCapacity: roomCapacity,
		idleRoomExpiry: idleRoomExpiry,
		lock:           &sync.RWMutex{},
	}
}
