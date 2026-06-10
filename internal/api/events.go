package api

import (
	"encoding/json"
	"sync"
)

// EventHub is a tiny fan-out bus for run lifecycle events, consumed by the
// browser over /ws/events so lists and dashboards update live.
type EventHub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// NewEventHub constructs an EventHub.
func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[chan []byte]struct{})}
}

// Subscribe registers a listener; call the returned func to unsubscribe.
func (h *EventHub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
}

// Publish marshals v and best-effort fans it out to all subscribers.
func (h *EventHub) Publish(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	for ch := range h.subs {
		select {
		case ch <- b:
		default: // drop for a slow consumer rather than block the publisher
		}
	}
	h.mu.Unlock()
}
