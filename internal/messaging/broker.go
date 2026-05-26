package messaging

import "sync"

// Event is the kind of notification pushed to a subscriber's SSE stream.
type Event string

const (
	// EventMessage signals a new message (client should reload messages).
	EventMessage Event = "update"
	// EventTyping signals the other user is typing (client shows the typing
	// indicator and must NOT reload messages).
	EventTyping Event = "typing"
)

// sseDataLine returns the SSE wire line for an event kind. The chat-drawer
// client distinguishes events by this data payload ("update" vs "typing").
func sseDataLine(ev Event) string {
	return "data: " + string(ev) + "\n\n"
}

type Broker struct {
	mu          sync.RWMutex
	subscribers map[string]chan Event
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string]chan Event),
	}
}

func (b *Broker) Subscribe(userID string) chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	if old, ok := b.subscribers[userID]; ok {
		close(old)
	}

	ch := make(chan Event, 8)
	b.subscribers[userID] = ch
	return ch
}

func (b *Broker) Unsubscribe(userID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[userID]; ok {
		close(ch)
		delete(b.subscribers, userID)
	}
}

// Notify signals a new message to userID.
func (b *Broker) Notify(userID string) {
	b.send(userID, EventMessage)
}

// NotifyTyping signals that someone is typing to userID. Distinct from Notify
// so the client can show the typing indicator without reloading messages.
func (b *Broker) NotifyTyping(userID string) {
	b.send(userID, EventTyping)
}

func (b *Broker) send(userID string, ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if ch, ok := b.subscribers[userID]; ok {
		select {
		case ch <- ev:
		default:
		}
	}
}
