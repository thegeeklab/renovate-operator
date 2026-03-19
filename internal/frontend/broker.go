package frontend

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	clientChanBufferSize = 20
	defaultPingInterval  = 15 * time.Second
)

// SSEBroker manages connected clients and broadcasts events.
type SSEBroker struct {
	clients      map[chan string]bool
	mu           sync.RWMutex
	pingInterval time.Duration
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients:      make(map[chan string]bool),
		pingInterval: defaultPingInterval,
	}
}

// SetPingInterval allows tests to override the default 15s heartbeat.
func (b *SSEBroker) SetPingInterval(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pingInterval = d
}

// Broadcast sends a named event to all connected browsers.
func (b *SSEBroker) Broadcast(eventName, data string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	safeData := strings.ReplaceAll(data, "\n", "\ndata: ")
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, safeData)

	for clientChan := range b.clients {
		select {
		case clientChan <- msg:
		default:
			// Drop the message if the client buffer is full
		}
	}
}

// ClientCount returns the number of connected clients.
func (b *SSEBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.clients)
}

// CloseAll disconnects all clients.
func (b *SSEBroker) CloseAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for clientChan := range b.clients {
		close(clientChan)
		delete(b.clients, clientChan)
	}
}

// ServeHTTP handles the SSE connection for a client.
func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "event: connected\ndata: ok\n\n")
	fmt.Fprint(w, ": heartbeat\n\n")
	flusher.Flush()

	clientChan := make(chan string, clientChanBufferSize)

	b.mu.Lock()
	b.clients[clientChan] = true
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		if _, exists := b.clients[clientChan]; exists {
			delete(b.clients, clientChan)
			close(clientChan)
		}
		b.mu.Unlock()
	}()

	b.mu.RLock()
	interval := b.pingInterval
	b.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}

			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, "event: ping\ndata: ok\n\n")
			flusher.Flush()
		}
	}
}
