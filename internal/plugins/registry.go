package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"

	"gopkg.in/yaml.v3"
)

// Resolved is a Manifest together with the name of the source that provided it.
type Resolved struct {
	Manifest Manifest
	Source   string // "file" | "db" | "crd"
}

// Registry is a read-only view over a set of plugin manifests with change notification.
type Registry interface {
	Get(id string) (Resolved, bool)
	List() []Resolved
	// Subscribe returns a channel that receives an empty struct on every change.
	Subscribe() <-chan struct{}
	// Run starts background processing; returns nil immediately, goroutines stop on ctx.Done().
	Run(ctx context.Context) error
}

// MergedRegistry fans in multiple registries with precedence — index 0 is the highest.
type MergedRegistry struct {
	sources []Registry
	logger  *slog.Logger

	mu            sync.RWMutex
	resolved      map[string]Resolved
	loggedShadows map[string]struct{} // dedupe shadow-log events, guarded by mu

	subscribersMu sync.Mutex
	subscribers   []chan struct{}

	once sync.Once
}

// NewMerged creates a MergedRegistry.  sources must be ordered highest-precedence first
// (e.g. CRD, DB, File).
func NewMerged(logger *slog.Logger, sources ...Registry) *MergedRegistry {
	return &MergedRegistry{
		sources:       sources,
		logger:        logger,
		resolved:      make(map[string]Resolved),
		loggedShadows: make(map[string]struct{}),
	}
}

// Get returns the highest-precedence Resolved for the given id.
func (m *MergedRegistry) Get(id string) (Resolved, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.resolved[id]
	return r, ok
}

// List returns a snapshot copy of all current resolved entries.
func (m *MergedRegistry) List() []Resolved {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Resolved, 0, len(m.resolved))
	for _, r := range m.resolved {
		out = append(out, r)
	}
	return out
}

// Subscribe returns a buffered (cap 1) channel that receives an empty struct on each change.
// Sends are non-blocking; slow consumers simply miss coalescence.
func (m *MergedRegistry) Subscribe() <-chan struct{} {
	ch := make(chan struct{}, 1)
	m.subscribersMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subscribersMu.Unlock()
	return ch
}

// Run starts the background goroutines under a sync.Once (idempotent).
// It performs an initial rebuild before returning so Get/List are usable immediately.
func (m *MergedRegistry) Run(ctx context.Context) error {
	m.once.Do(func() {
		// Initial population before anything subscribes upstream.
		m.rebuild()

		for _, src := range m.sources {
			src := src
			// Start the source's own background work.
			go func() { _ = src.Run(ctx) }()

			// Subscribe to that source and rebuild on every notification.
			ch := src.Subscribe()
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-ch:
						if !ok {
							return
						}
						m.rebuild()
						m.broadcast()
					}
				}
			}()
		}
	})
	return nil
}

// rebuild recomputes the merged map from all sources and logs new shadow events.
// Highest-precedence source wins; lower-precedence entries with different content are shadowed.
func (m *MergedRegistry) rebuild() {
	// Per-id we record all (source, hash) pairs seen during the sweep.
	type entry struct {
		resolved Resolved
		hash     string
	}

	// allEntries maps id → ordered list of (resolved, hash), highest-prec first.
	allEntries := make(map[string][]entry)

	for _, src := range m.sources {
		for _, r := range src.List() {
			id := r.Manifest.Spec.ID
			h := specHash(r.Manifest.Spec)
			allEntries[id] = append(allEntries[id], entry{resolved: r, hash: h})
		}
	}

	newMap := make(map[string]Resolved, len(allEntries))
	for id, entries := range allEntries {
		newMap[id] = entries[0].resolved
	}

	// Compute shadow events to log (detect after lock acquisition to keep consistent).
	type shadowEvent struct {
		id, winning, shadowed, winHash, shadowHash string
	}
	var toLog []shadowEvent

	m.mu.Lock()
	m.resolved = newMap

	for id, entries := range allEntries {
		if len(entries) < 2 {
			continue
		}
		winner := entries[0]
		for _, loser := range entries[1:] {
			if loser.hash == winner.hash {
				continue
			}
			key := fmt.Sprintf("%s|%s|%s|%s|%s",
				id, winner.resolved.Source, loser.resolved.Source, winner.hash, loser.hash)
			if _, already := m.loggedShadows[key]; already {
				continue
			}
			m.loggedShadows[key] = struct{}{}
			toLog = append(toLog, shadowEvent{
				id:         id,
				winning:    winner.resolved.Source,
				shadowed:   loser.resolved.Source,
				winHash:    winner.hash,
				shadowHash: loser.hash,
			})
		}
	}
	m.mu.Unlock()

	// Emit warnings outside the lock.
	for _, ev := range toLog {
		m.logger.Warn("plugin shadowed",
			"id", ev.id,
			"winning", ev.winning,
			"shadowed", ev.shadowed,
			"winning_hash", ev.winHash,
			"shadowed_hash", ev.shadowHash,
		)
	}
}

// broadcast sends an empty struct to every subscriber channel without blocking.
func (m *MergedRegistry) broadcast() {
	m.subscribersMu.Lock()
	subs := make([]chan struct{}, len(m.subscribers))
	copy(subs, m.subscribers)
	m.subscribersMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// specHash returns a short hex SHA-256 of the yaml-marshaled ManifestSpec.
func specHash(spec ManifestSpec) string {
	b, _ := yaml.Marshal(spec)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8]) // 16 hex chars is enough for dedup
}
