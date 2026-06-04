package plugins

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileRegistry reads *.yaml / *.yml files from a directory, parses them as plugin
// manifests, and hot-reloads via fsnotify.
type FileRegistry struct {
	baseRegistry

	dir    string
	logger *slog.Logger

	pathToID map[string]string  // last-known plugin id per abs-path, for delete handling
	pathLast map[string]Resolved // cached last-valid Resolved per abs-path (malformed-keeps-prev)
}

// NewFileRegistry creates a new FileRegistry that watches dir.
// Neither directory existence nor format errors are fatal at construction time.
func NewFileRegistry(dir string, logger *slog.Logger) *FileRegistry {
	return &FileRegistry{
		baseRegistry: baseRegistry{resolved: make(map[string]Resolved)},
		dir:          dir,
		logger:       logger,
		pathToID:     make(map[string]string),
		pathLast:     make(map[string]Resolved),
	}
}

// Run performs an initial directory scan then watches for changes via fsnotify.
// It is idempotent (sync.Once). The function returns nil immediately; all
// blocking work runs in background goroutines that stop when ctx is cancelled.
// A missing directory or watcher failure is logged as WARN but is not an error.
func (r *FileRegistry) Run(ctx context.Context) error {
	r.once.Do(func() {
		// 1. Initial scan — populate resolved before returning.
		r.scan()

		// 2. Attempt to start the file watcher.
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			r.logger.Warn("file_registry: fsnotify.NewWatcher failed", "err", err)
			return
		}

		if err := watcher.Add(r.dir); err != nil {
			r.logger.Warn("file_registry: plugin dir not found or unreadable",
				"dir", r.dir, "err", err)
			_ = watcher.Close()
			return
		}

		// 3. Background goroutine: debounce events → rescan → broadcast.
		go r.watch(ctx, watcher)
	})
	return nil
}

// watch is the background event-loop for the fsnotify watcher.
func (r *FileRegistry) watch(ctx context.Context, watcher *fsnotify.Watcher) {
	defer watcher.Close()

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return

		case _, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Reset the debounce timer on every event.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(200 * time.Millisecond)
			timerC = timer.C

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			r.logger.Warn("file_registry: watcher error", "err", err)

		case <-timerC:
			// Debounce fired — rescan and notify subscribers.
			timerC = nil
			r.scan()
			r.broadcast()
		}
	}
}

// scan reads all *.yaml / *.yml files from r.dir, parses them, updates r.resolved
// atomically, and maintains the pathLast / pathToID caches for malformed-keeps-prev
// and delete handling.
func (r *FileRegistry) scan() {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		// Dir might not exist; already warned at Run time; keep existing state.
		return
	}

	// Collect all eligible paths in this sweep.
	currentPaths := make(map[string]struct{})

	// Build a fresh resolved map locally so the swap is atomic.
	newResolved := make(map[string]Resolved)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip dot-files (.swp, .DS_Store, etc.)
		if strings.HasPrefix(name, ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		absPath := filepath.Join(r.dir, name)
		currentPaths[absPath] = struct{}{}

		data, err := os.ReadFile(absPath)
		if err != nil {
			r.logger.Warn("file_registry: cannot read file", "path", absPath, "err", err)
			// Fall back to previous valid entry for this path, if any.
			r.mu.RLock()
			prev, hasPrev := r.pathLast[absPath]
			r.mu.RUnlock()
			if hasPrev {
				newResolved[prev.Manifest.Spec.ID] = prev
			}
			continue
		}

		m, err := ParseManifest(data)
		if err != nil {
			r.logger.Warn("file_registry: manifest parse error", "path", absPath, "err", err)
			// Keep previous valid entry for this path.
			r.mu.RLock()
			prev, hasPrev := r.pathLast[absPath]
			r.mu.RUnlock()
			if hasPrev {
				newResolved[prev.Manifest.Spec.ID] = prev
			}
			continue
		}

		resolved := Resolved{Manifest: *m, Source: "file"}
		newResolved[m.Spec.ID] = resolved

		// Update caches (will be committed to r.* under the write lock below).
		r.mu.Lock()
		r.pathLast[absPath] = resolved
		r.pathToID[absPath] = m.Spec.ID
		r.mu.Unlock()
	}

	// Drop pathLast / pathToID entries for files that are no longer present.
	r.mu.Lock()
	for path := range r.pathLast {
		if _, present := currentPaths[path]; !present {
			delete(r.pathLast, path)
			delete(r.pathToID, path)
		}
	}

	// Atomic swap of the resolved map.
	r.resolved = newResolved
	r.mu.Unlock()
}
