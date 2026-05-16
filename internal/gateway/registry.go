package gateway

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
)

var ErrRuntimeNotConnected = errors.New("runtime not connected")

type RuntimeRegistry struct {
	mu       sync.Mutex
	runtimes map[string]*RuntimeConnection
	waiters  map[waiterKey]*waiter
}

type waiterKey struct {
	agentID   string
	messageID string
}

type waiter struct {
	events chan Envelope
	done   chan struct{}
	mu     sync.Mutex
	once   sync.Once
}

func newWaiter() *waiter {
	return &waiter{events: make(chan Envelope, 16), done: make(chan struct{})}
}

func (w *waiter) cancel() {
	w.once.Do(func() { close(w.done) })
}

func (w *waiter) deliver(event Envelope) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return false
	case w.events <- event:
		return true
	}
}

func (w *waiter) fail(event Envelope) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return
	case w.events <- event:
	}
	w.cancel()
}

type RuntimeConnection struct {
	AgentID string
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{runtimes: map[string]*RuntimeConnection{}, waiters: map[waiterKey]*waiter{}}
}

func (r *RuntimeRegistry) Register(agentID string, conn *websocket.Conn) {
	r.mu.Lock()
	previous := r.runtimes[agentID]
	r.runtimes[agentID] = &RuntimeConnection{AgentID: agentID, conn: conn}
	defer r.mu.Unlock()
	if previous != nil && previous.conn != conn {
		_ = previous.conn.Close()
		r.failWaitersLocked(agentID, "runtime connection replaced")
	}
}

func (r *RuntimeRegistry) Unregister(agentID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.runtimes[agentID]; ok && current.conn == conn {
		delete(r.runtimes, agentID)
		r.failWaitersLocked(agentID, "runtime disconnected")
	}
}

func (r *RuntimeRegistry) SendTask(agentID string, task Envelope) (<-chan Envelope, func(), error) {
	r.mu.Lock()
	runtime := r.runtimes[agentID]
	if runtime == nil {
		r.mu.Unlock()
		return nil, nil, ErrRuntimeNotConnected
	}
	key := waiterKey{agentID: agentID, messageID: task.MessageID}
	w := newWaiter()
	r.waiters[key] = w
	r.mu.Unlock()

	runtime.writeMu.Lock()
	err := runtime.conn.WriteJSON(task)
	runtime.writeMu.Unlock()
	if err != nil {
		r.removeWaiter(key)
		return nil, nil, err
	}

	cancel := func() { r.removeWaiter(key) }
	return w.events, cancel, nil
}

func (r *RuntimeRegistry) Dispatch(agentID string, conn *websocket.Conn, event Envelope) {
	if event.AgentID != agentID {
		return
	}
	key := waiterKey{agentID: agentID, messageID: event.MessageID}
	r.mu.Lock()
	runtime := r.runtimes[agentID]
	w := r.waiters[key]
	r.mu.Unlock()
	if runtime == nil || runtime.conn != conn || w == nil {
		return
	}
	if !w.deliver(event) {
		return
	}
	if event.Type == "message.done" || event.Type == "message.error" {
		r.removeWaiter(key)
	}
}

func (r *RuntimeRegistry) failWaitersLocked(agentID, reason string) {
	for key, w := range r.waiters {
		if key.agentID != agentID {
			continue
		}
		delete(r.waiters, key)
		w.fail(Envelope{Type: "message.error", AgentID: agentID, MessageID: key.messageID, Payload: map[string]any{"text": reason}})
	}
}

func (r *RuntimeRegistry) removeWaiter(key waiterKey) {
	r.mu.Lock()
	w := r.waiters[key]
	delete(r.waiters, key)
	r.mu.Unlock()
	if w != nil {
		w.cancel()
	}
}
