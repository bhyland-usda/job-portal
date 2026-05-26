package feed

import "sync"

type Broker struct {
	mu          sync.RWMutex
	subscribers map[string]chan string
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string]chan string),
	}
}

func (b *Broker) Subscribe(userID string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if old, ok := b.subscribers[userID]; ok {
		close(old)
	}

	ch := make(chan string, 4)
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

// Broadcast sends a post ID to all subscribers.
// Use "" for general refresh (new post created, post deleted).
func (b *Broker) Broadcast(postID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- postID:
		default:
		}
	}
}
