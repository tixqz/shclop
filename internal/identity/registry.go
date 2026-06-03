package identity

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

type ProviderStatus string

const (
	StatusReady    ProviderStatus = "ready"
	StatusDegraded ProviderStatus = "degraded"
)

// MaterializedProvider is an entry in the registry.
type MaterializedProvider struct {
	Config   OIDCProviderConfig
	Provider *OIDCProvider
	Status   ProviderStatus
	LastErr  string
	Enabled  bool
}

// SettingsReader is the narrow Store interface the registry depends on.
type SettingsReader interface {
	GetAppSetting(ctx context.Context, key string) (string, error)
	ListAppSettings(ctx context.Context, prefix string) (map[string]string, error)
}

// IdPRegistry manages materialized OIDC providers with overlay and retry.
type IdPRegistry interface {
	Get(name string) (*MaterializedProvider, bool)
	List() []*MaterializedProvider
	Mode() domain.AuthMode
	Subscribe() <-chan struct{}
	Reload(ctx context.Context) error
	Run(ctx context.Context) error
	Close()
}

type idpRegistry struct {
	configs       []OIDCProviderConfig
	settings      SettingsReader
	retryInterval time.Duration
	logger        *slog.Logger

	mu       sync.RWMutex
	snapshot map[string]*MaterializedProvider
	mode     domain.AuthMode

	subscribersMu sync.Mutex
	subscribers   []chan struct{}

	once sync.Once
}

// NewIdPRegistry materializes all providers from configs synchronously.
// Failed materializations are stored as degraded; only systemic errors are returned.
func NewIdPRegistry(ctx context.Context, configs []OIDCProviderConfig, settings SettingsReader, retryInterval time.Duration, logger *slog.Logger) (IdPRegistry, error) {
	if settings == nil {
		return nil, errors.New("settings must not be nil")
	}
	if retryInterval == 0 {
		retryInterval = 60 * time.Second
	}

	r := &idpRegistry{
		configs:       configs,
		settings:      settings,
		retryInterval: retryInterval,
		logger:        logger,
		snapshot:      make(map[string]*MaterializedProvider),
	}

	for _, cfg := range configs {
		tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p, err := NewOIDCProvider(tctx, cfg)
		cancel()

		entry := &MaterializedProvider{Config: cfg}
		if err != nil {
			entry.Status = StatusDegraded
			entry.LastErr = err.Error()
			logger.Warn("idp_registry: provider degraded at startup", "name", cfg.Name, "err", err)
		} else {
			entry.Status = StatusReady
			entry.Provider = p
		}
		r.snapshot[cfg.Name] = entry
	}

	if err := r.applyOverlay(ctx); err != nil {
		logger.Warn("idp_registry: overlay read failed at startup", "err", err)
	}

	return r, nil
}

// applyOverlay reads auth.mode and per-provider enabled flags from settings.
// Must be called with write lock held OR before the registry is shared (startup).
// Returns error only if the store call fails.
func (r *idpRegistry) applyOverlay(ctx context.Context) error {
	modeVal, err := r.settings.GetAppSetting(ctx, "auth.mode")
	if err != nil {
		return err
	}
	switch domain.AuthMode(modeVal) {
	case domain.AuthModeSSO, domain.AuthModeBoth, domain.AuthModeLocal:
		r.mode = domain.AuthMode(modeVal)
	default:
		r.mode = domain.AuthModeLocal
	}

	overlay, err := r.settings.ListAppSettings(ctx, "auth.idp.")
	if err != nil {
		return err
	}
	for name, entry := range r.snapshot {
		key := "auth.idp." + name + ".enabled"
		if val, ok := overlay[key]; ok {
			entry.Enabled = val == "true"
		} else {
			entry.Enabled = true
		}
	}
	return nil
}

func (r *idpRegistry) Get(name string) (*MaterializedProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.snapshot[name]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

func (r *idpRegistry) List() []*MaterializedProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*MaterializedProvider, 0, len(r.snapshot))
	for _, e := range r.snapshot {
		cp := *e
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Config.Name < out[j].Config.Name
	})
	return out
}

func (r *idpRegistry) Mode() domain.AuthMode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
}

func (r *idpRegistry) Subscribe() <-chan struct{} {
	ch := make(chan struct{}, 1)
	r.subscribersMu.Lock()
	r.subscribers = append(r.subscribers, ch)
	r.subscribersMu.Unlock()
	return ch
}

// Reload refreshes the overlay and mode from the store without re-attempting degraded providers.
func (r *idpRegistry) Reload(ctx context.Context) error {
	modeVal, err := r.settings.GetAppSetting(ctx, "auth.mode")
	if err != nil {
		return err
	}
	overlay, err := r.settings.ListAppSettings(ctx, "auth.idp.")
	if err != nil {
		return err
	}

	r.mu.Lock()
	switch domain.AuthMode(modeVal) {
	case domain.AuthModeSSO, domain.AuthModeBoth, domain.AuthModeLocal:
		r.mode = domain.AuthMode(modeVal)
	default:
		r.mode = domain.AuthModeLocal
	}
	for name, entry := range r.snapshot {
		key := "auth.idp." + name + ".enabled"
		if val, ok := overlay[key]; ok {
			entry.Enabled = val == "true"
		} else {
			entry.Enabled = true
		}
	}
	r.mu.Unlock()

	r.broadcast()
	return nil
}

// Run starts the background degraded-retry loop. Idempotent via sync.Once.
func (r *idpRegistry) Run(ctx context.Context) error {
	r.once.Do(func() {
		go func() {
			ticker := time.NewTicker(r.retryInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					r.retryDegraded(ctx)
				}
			}
		}()
	})
	return nil
}

func (r *idpRegistry) retryDegraded(ctx context.Context) {
	r.mu.RLock()
	var toRetry []string
	for name, e := range r.snapshot {
		if e.Status == StatusDegraded {
			toRetry = append(toRetry, name)
		}
	}
	r.mu.RUnlock()

	if len(toRetry) == 0 {
		return
	}

	changed := false
	for _, name := range toRetry {
		r.mu.RLock()
		entry, ok := r.snapshot[name]
		r.mu.RUnlock()
		if !ok {
			continue
		}

		tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p, err := NewOIDCProvider(tctx, entry.Config)
		cancel()

		if err != nil {
			r.logger.Warn("idp_registry: retry failed", "name", name, "err", err)
			continue
		}

		r.mu.Lock()
		if e, exists := r.snapshot[name]; exists && e.Status == StatusDegraded {
			e.Provider = p
			e.Status = StatusReady
			e.LastErr = ""
			changed = true
			r.logger.Info("idp_registry: provider recovered", "name", name)
		}
		r.mu.Unlock()
	}

	if changed {
		r.broadcast()
	}
}

func (r *idpRegistry) Close() {}

func (r *idpRegistry) broadcast() {
	r.subscribersMu.Lock()
	subs := make([]chan struct{}, len(r.subscribers))
	copy(subs, r.subscribers)
	r.subscribersMu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// compile-time interface check
var _ IdPRegistry = (*idpRegistry)(nil)
