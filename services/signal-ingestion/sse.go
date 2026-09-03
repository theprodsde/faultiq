package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/redis/go-redis/v9"
)

// SSEHub manages Server-Sent Event connections and broadcasts incident events from Redis pub/sub.
type SSEHub struct {
	rdb     *redis.Client
	clients map[chan string]struct{}
	mu      sync.RWMutex
}

func NewSSEHub(rdb *redis.Client) *SSEHub {
	return &SSEHub{
		rdb:     rdb,
		clients: make(map[chan string]struct{}),
	}
}

// Run subscribes to the "incident_events" Redis channel and broadcasts to all connected SSE clients.
func (h *SSEHub) Run(ctx context.Context) {
	sub := h.rdb.Subscribe(ctx, "incident_events", "signals")
	ch := sub.Channel()
	log.Println("sse-hub: subscribed to incident_events and signals channels")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Wrap in SSE format with event type from channel name
			eventType := "message"
			if msg.Channel == "incident_events" {
				eventType = "incident"
			} else if msg.Channel == "signals" {
				eventType = "signal"
			}
			sseData := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, msg.Payload)
			h.broadcast(sseData)
		}
	}
}

func (h *SSEHub) broadcast(data string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// Client too slow, skip
		}
	}
}

func (h *SSEHub) addClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

func (h *SSEHub) removeClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
	close(ch)
}

// HandleSSE is the HTTP handler for /api/v1/events (Server-Sent Events endpoint).
func (h *SSEHub) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

	clientCh := make(chan string, 64)
	h.addClient(clientCh)
	defer h.removeClient(clientCh)

	// Send initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-clientCh:
			if !ok {
				return
			}
			fmt.Fprint(w, data)
			flusher.Flush()
		}
	}
}
