package mem

import (
	"encoding/json"
	"sort"

	"github.com/bnkamalesh/phispr/internal/messages"
)

type Store struct {
	// capacity decides how many messages are to be retained.
	// if exceeded, messages will be evicted in oldest first order
	capacity uint
	data     []messages.Message
	queue    chan<- *messages.Message
}

func (str *Store) Add(msg *messages.Message) error {
	str.queue <- msg
	return nil
}

func (str *Store) add(msg *messages.Message) {
	if len(str.data) >= int(str.capacity) {
		str.data = str.data[1:] // Remove the oldest message
	}
	str.data = append(str.data, *msg)
}

func (str *Store) All() []messages.Message {
	descendingMessages := make([]messages.Message, len(str.data))
	copy(descendingMessages, str.data)
	sort.Slice(descendingMessages, func(i, j int) bool {
		return descendingMessages[i].PublishedAt.After(descendingMessages[j].PublishedAt)
	})

	return descendingMessages
}

func (str *Store) listener(queue <-chan *messages.Message) {
	for msg := range queue {
		str.add(msg)
	}
}

func (str *Store) UnmarshalJSON(payload []byte) error {
	all := []messages.Message{}
	err := json.Unmarshal(payload, &all)
	str.data = all
	return err
}

func (str *Store) MarshalJSON() ([]byte, error) {
	return json.Marshal(str.data)
}

func New(capacity uint) *Store {
	queue := make(chan *messages.Message, capacity)
	str := &Store{
		capacity: capacity,
		data:     make([]messages.Message, 0, capacity),
		queue:    queue,
	}

	go str.listener(queue)

	return str
}
