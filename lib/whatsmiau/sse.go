package whatsmiau

import "sync"

type StatusEvent struct {
	Instance string `json:"instance"`
	State    string `json:"state"`
	Wuid     string `json:"wuid,omitempty"`
}

type SSEBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan StatusEvent]struct{}
}

func NewSSEBroadcaster() *SSEBroadcaster {
	return &SSEBroadcaster{
		clients: make(map[chan StatusEvent]struct{}),
	}
}

func (b *SSEBroadcaster) Register() chan StatusEvent {
	ch := make(chan StatusEvent, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *SSEBroadcaster) Unregister(ch chan StatusEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *SSEBroadcaster) Broadcast(event StatusEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default: // skip slow clients
		}
	}
}
