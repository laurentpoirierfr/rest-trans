package notification

import (
	"log/slog"
	"sync"
)

type Event struct {
	Channel string
	Payload string
}

type Client struct {
	C    chan Event
	quit chan struct{}
}

func NewClient() *Client {
	return &Client{
		C:    make(chan Event, 64),
		quit: make(chan struct{}),
	}
}

func (c *Client) Close() {
	select {
	case <-c.quit:
	default:
		close(c.quit)
	}
}

func (c *Client) Done() <-chan struct{} {
	return c.quit
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Subscribe(channel string) *Client {
	client := NewClient()

	h.mu.Lock()
	if h.clients[channel] == nil {
		h.clients[channel] = make(map[*Client]struct{})
	}
	h.clients[channel][client] = struct{}{}
	h.mu.Unlock()

	slog.Info("sse client subscribed", "channel", channel)
	return client
}

func (h *Hub) Unsubscribe(channel string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, channel)
		}
	}
	client.Close()
	slog.Info("sse client unsubscribed", "channel", channel)
}

func (h *Hub) Broadcast(event Event) {
	h.mu.RLock()
	clients := make([]*Client, 0)
	for c := range h.clients[event.Channel] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.C <- event:
		case <-c.quit:
			h.Unsubscribe(event.Channel, c)
		default:
			h.Unsubscribe(event.Channel, c)
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}
