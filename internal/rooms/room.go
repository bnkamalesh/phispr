package rooms

import (
	"sort"
	"strings"
	"time"

	"github.com/bnkamalesh/phispr/internal/messages"
	"github.com/bnkamalesh/phispr/internal/messages/stores/mem"
	"github.com/bnkamalesh/phispr/internal/users"
	"github.com/gosimple/slug"
	"github.com/naughtygopher/errors"
)

type Room struct {
	ID       string
	Name     string
	Capacity uint
	// map[name]tokenID
	nameUsed map[string]string
	Members  map[string]*Member
	Owner    *Member
	// SecretHashKey is used to create a authn token per member
	SecretHashKey string
	msgs          *messages.Messages
	Listed        bool
	// Phantom if enabled would not retain messages on the server
	Phantom        bool
	CreatedAt      time.Time
	LastActivityAt time.Time
	Expiry         time.Duration
}

func (rm *Room) Sanitize() {
	rm.Name = strings.TrimSpace(rm.Name)
	if rm.Capacity < roomCapacity {
		rm.Capacity = roomCapacity
	}

	if rm.Expiry < time.Minute {
		rm.Expiry = time.Minute
	}

	rm.ID = slug.Make(rm.Name)

	if rm.Members == nil {
		rm.Members = make(map[string]*Member, rm.Capacity)
	}

	if rm.nameUsed == nil {
		rm.nameUsed = make(map[string]string, rm.Capacity)
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

func (rm *Room) IsOwner(mem *Member) bool {
	if rm.Owner == nil || mem == nil {
		return false
	}

	return rm.Owner.User.ID == mem.User.ID
}

func (rm *Room) HasMember(memberToken string) bool {
	_, exists := rm.Members[memberToken]
	return exists
}

func (rm *Room) AddMember(usr *users.User, memberToken string) (*Member, error) {
	_, exists := rm.nameUsed[usr.ID]
	if exists {
		return nil, errors.Duplicatef("member %q is already in the room %q", usr.Name, rm.ID)
	}

	if len(rm.Members)+1 > int(rm.Capacity) {
		return nil, errors.Errorf("room %q already has %d members", rm.ID, len(rm.Members))
	}

	member := &Member{
		Token:  memberToken,
		User:   usr,
		RoomID: rm.ID,
	}

	rm.Members[member.Token] = member
	rm.nameUsed[usr.ID] = member.Token
	// a newly joined member can become the owner if there are no other members
	if len(rm.Members) <= 1 {
		rm.Owner = member
	}

	rm.LastActivityAt = time.Now()

	return member, nil
}

func (rm *Room) RemoveMember(username string) (*Member, error) {
	username = strings.TrimSpace(username)
	userID := slug.Make(username)
	tokenID, exists := rm.nameUsed[userID]
	if !exists {
		return nil, errors.Duplicatef(
			"user %q is not a member of the room %q",
			username, rm.ID,
		)
	}
	member := rm.Members[tokenID]

	delete(rm.Members, member.Token)
	delete(rm.nameUsed, member.User.ID)

	return member, nil
}

func (rm *Room) RemoveMemberAutoAssignOwner(username string) (*Member, error) {
	removedMember, err := rm.RemoveMember(username)
	if err != nil {
		return nil, err
	}

	// auto assign *earliest member as owner*
	var earliestMember *Member
	for key := range rm.Members {
		member := rm.Members[key]
		if earliestMember == nil {
			earliestMember = member
			continue
		}
		if member.User.Joined.Before(earliestMember.User.Joined) {
			earliestMember = member
		}
	}

	rm.Owner = earliestMember

	return removedMember, nil
}

func (rm *Room) Messages() []messages.Message {
	return rm.msgs.All()
}

func (rm *Room) MessageRetentionCapacity() uint {
	return rm.msgs.Capacity()
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

func (rm *Room) initMessages() error {
	memstore := mem.New(rm.Capacity * 3)
	var err error
	rm.msgs, err = messages.NewMessages(rm.Capacity*3, memstore)
	return err
}

func (rm *Room) cleanupMembers() {
	for key := range rm.Members {
		member := rm.Members[key]
		if time.Since(member.User.LastPing) > rm.Expiry &&
			time.Since(member.User.LastMessage) > rm.Expiry {
			rm.RemoveMemberAutoAssignOwner(member.User.Name)
		}
	}
}

func (rm *Room) Cleanup() bool {
	rm.cleanupMembers()

	if len(rm.Members) != 0 {
		return false
	}

	if time.Since(rm.LastActivityAt) < rm.Expiry {
		return false
	}

	return true
}

func (rm *Room) MemberByUserID(userID string) (*Member, error) {
	member, found := rm.nameUsed[userID]
	if !found {
		return nil, errors.NotFoundf("member %q not found in room %q", userID, rm.ID)
	}

	return rm.Members[member], nil
}

func NewRoom(id, name string, listed, phantom bool, capacity uint, expiry time.Duration) (*Room, error) {
	now := time.Now()
	room := &Room{
		ID:             id,
		Name:           name,
		Capacity:       capacity,
		Members:        make(map[string]*Member, capacity),
		Listed:         listed,
		CreatedAt:      now,
		LastActivityAt: now,
		Phantom:        phantom,
		Expiry:         expiry,
	}

	err := room.SanitizeValidate()
	if err != nil {
		return nil, err
	}

	err = room.initMessages()
	if err != nil {
		return nil, err
	}

	return room, nil
}
