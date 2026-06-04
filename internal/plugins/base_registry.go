package plugins

import "sync"

// baseRegistry implements the lock-protected resolved map and pub/sub
// behaviour shared by FileRegistry, DBRegistry, and CRDRegistry.
// It is meant to be embedded; do not construct directly.
type baseRegistry struct {
	mu       sync.RWMutex
	resolved map[string]Resolved

	subscribersMu sync.Mutex
	subscribers   []chan struct{}

	once sync.Once
}

// Get returns the Resolved manifest for the given plugin id, if present.
func (b *baseRegistry) Get(id string) (Resolved, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	v, ok := b.resolved[id]
	return v, ok
}

// List returns a snapshot copy of all currently resolved entries.
func (b *baseRegistry) List() []Resolved {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Resolved, 0, len(b.resolved))
	for _, v := range b.resolved {
		out = append(out, v)
	}
	return out
}

// Subscribe returns a buffered (cap 1) channel that receives an empty struct on each change.
// Sends are non-blocking; slow consumers simply miss coalescence.
func (b *baseRegistry) Subscribe() <-chan struct{} {
	ch := make(chan struct{}, 1)
	b.subscribersMu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.subscribersMu.Unlock()
	return ch
}

// broadcast sends an empty struct to every subscriber channel without blocking.
func (b *baseRegistry) broadcast() {
	b.subscribersMu.Lock()
	subs := make([]chan struct{}, len(b.subscribers))
	copy(subs, b.subscribers)
	b.subscribersMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
