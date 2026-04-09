package whatsmiau

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
)

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

func (b *SSEBroadcaster) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)
		c.Response().Flush()

		ch := b.Register()
		defer b.Unregister(ch)

		for {
			select {
			case event := <-ch:
				data, _ := json.Marshal(event)
				fmt.Fprintf(c.Response(), "event: status\ndata: %s\n\n", data)
				c.Response().Flush()
			case <-c.Request().Context().Done():
				return nil
			}
		}
	}
}
