package notification

import "sync"

type Broker struct {
	mu      sync.RWMutex
	clients map[string]map[chan string]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[chan string]struct{}),
	}
}

func (b *Broker) Subscribe(userID string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan string, 1)
	if b.clients[userID] == nil {
		b.clients[userID] = make(map[chan string]struct{})
	}
	b.clients[userID][ch] = struct{}{}
	return ch
}

func (b *Broker) Unsubscribe(userID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.clients[userID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(b.clients, userID)
		}
	}
	close(ch)
}

func (b *Broker) Notify(userID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.clients[userID]; ok {
		for ch := range subs {
			select {
			case ch <- "notification":
			default:
			}
		}
	}
}
