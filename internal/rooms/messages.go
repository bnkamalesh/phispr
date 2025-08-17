package rooms

import (
	"sort"

	"github.com/bnkamalesh/chat/internal/messages"
)

type Messages struct {
	capacity uint
	queue    chan<- *messages.Message
	// Phantom if true, messages are not retained on server at all
	Phantom  bool
	Messages []messages.Message
}

func (msgs *Messages) Add(msg *messages.Message) error {
	if msgs.Phantom {
		return nil
	}
	msgs.queue <- msg
	return nil
}

func (msgs *Messages) add(msg *messages.Message) {
	if len(msgs.Messages) >= int(msgs.capacity) {
		msgs.Messages = msgs.Messages[1:] // Remove the oldest message
	}
	msgs.Messages = append(msgs.Messages, *msg)
}

func (msgs *Messages) All() []messages.Message {
	descendingMessages := make([]messages.Message, len(msgs.Messages))
	copy(descendingMessages, msgs.Messages)
	sort.Slice(descendingMessages, func(i, j int) bool {
		return descendingMessages[i].PublishedAt.After(descendingMessages[j].PublishedAt)
	})

	return descendingMessages
}

func (msgs *Messages) listener(queue <-chan *messages.Message) {
	for msg := range queue {
		msgs.add(msg)
	}
}

func NewMessages(capacity uint, phantom bool) *Messages {
	queue := make(chan *messages.Message, capacity)
	msgs := &Messages{
		capacity: capacity,
		Messages: make([]messages.Message, 0, capacity),
		queue:    queue,
		Phantom:  phantom,
	}

	go msgs.listener(queue)

	return msgs
}
