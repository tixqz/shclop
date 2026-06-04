package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

// ManifestLister is the narrow interface DBRegistry needs from the store.
type ManifestLister interface {
	ListPluginManifests(ctx context.Context, enabledOnly bool) ([]domain.PluginManifest, error)
}

// DBRegistry polls the plugin_manifests table at a configurable interval,
// parses each row as a plugin manifest, and hot-reloads on changes.
type DBRegistry struct {
	baseRegistry

	store    ManifestLister
	interval time.Duration
	logger   *slog.Logger
	lastHash string
}

// NewDBRegistry creates a new DBRegistry. If interval is zero, it defaults to 10s.
func NewDBRegistry(store ManifestLister, interval time.Duration, logger *slog.Logger) *DBRegistry {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &DBRegistry{
		baseRegistry: baseRegistry{resolved: make(map[string]Resolved)},
		store:        store,
		interval:     interval,
		logger:       logger,
	}
}

// Run performs an initial synchronous poll then starts a background ticker.
// It is idempotent (sync.Once). Returns nil immediately; background work stops on ctx.Done().
func (r *DBRegistry) Run(ctx context.Context) error {
	r.once.Do(func() {
		// Initial synchronous poll so List/Get are populated before Run returns.
		r.poll(ctx)

		go func() {
			ticker := time.NewTicker(r.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					r.poll(ctx)
				}
			}
		}()
	})
	return nil
}

// poll fetches manifests from the store, parses them, and updates resolved if the
// snapshot hash changed. On store error it logs WARN and returns without updating state.
func (r *DBRegistry) poll(ctx context.Context) {
	rows, err := r.store.ListPluginManifests(ctx, true)
	if err != nil {
		r.logger.Warn("db_registry: store error, retaining previous state", "err", err)
		return
	}

	// Read prior resolved under RLock for parse-error fallback.
	r.mu.RLock()
	prior := r.resolved
	r.mu.RUnlock()

	// Build new resolved map. Collect (id, revision) pairs for hash computation.
	newResolved := make(map[string]Resolved, len(rows))
	type idRev struct {
		id  string
		rev int64
	}
	entries := make([]idRev, 0, len(rows))

	for _, row := range rows {
		m, parseErr := ParseManifest([]byte(row.YAML))
		if parseErr != nil {
			r.logger.Warn("db_registry: manifest parse error, keeping previous",
				"id", row.ID, "err", parseErr)
			// Keep previous Resolved for this id, if any.
			if prev, ok := prior[row.ID]; ok {
				newResolved[row.ID] = prev
			}
			// Still include this id+revision in the hash so a bumped-revision
			// malformed row does trigger a broadcast (callers get the prior data
			// but the change is acknowledged).
			entries = append(entries, idRev{id: row.ID, rev: row.Revision})
			continue
		}
		resolved := Resolved{Manifest: *m, Source: "db"}
		newResolved[m.Spec.ID] = resolved
		entries = append(entries, idRev{id: m.Spec.ID, rev: row.Revision})
	}

	// Compute snapshot hash: sort ids, concat "id:revision" with ";", sha256 → hex.
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(fmt.Sprintf("%s:%d", e.id, e.rev))
	}
	sum := sha256.Sum256([]byte(sb.String()))
	hash := hex.EncodeToString(sum[:])

	// No-op if hash unchanged.
	r.mu.RLock()
	same := hash == r.lastHash
	r.mu.RUnlock()
	if same {
		return
	}

	// Atomic swap.
	r.mu.Lock()
	r.resolved = newResolved
	r.lastHash = hash
	r.mu.Unlock()

	r.broadcast()
}
