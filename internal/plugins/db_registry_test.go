package plugins

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

// ---------------------------------------------------------------------------
// fakeLister
// ---------------------------------------------------------------------------

type fakeLister struct {
	mu        sync.Mutex
	manifests []domain.PluginManifest
	err       error
}

func newFakeLister(initial ...domain.PluginManifest) *fakeLister {
	return &fakeLister{manifests: append([]domain.PluginManifest(nil), initial...)}
}

func (f *fakeLister) ListPluginManifests(_ context.Context, _ bool) ([]domain.PluginManifest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	// Return a copy so post-call mutations don't affect the caller.
	out := make([]domain.PluginManifest, len(f.manifests))
	copy(out, f.manifests)
	return out, nil
}

func (f *fakeLister) set(manifests ...domain.PluginManifest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.manifests = append([]domain.PluginManifest(nil), manifests...)
}

func (f *fakeLister) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// ---------------------------------------------------------------------------
// Helper — builds valid manifest YAML
// ---------------------------------------------------------------------------

func dbTestManifest(id, displayName string) string {
	return fmt.Sprintf(`apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: %s }
spec:
  id: %s
  display_name: %s
  kind: http_template
  scopes_supported: [user]
  auth: { type: pat_token, fields: [{ name: token, label: Token, secret: true }] }
  validate:
    http:
      method: GET
      url: "https://api.example.com/me"
      headers: { Authorization: "Bearer {{.Token}}" }
      success_status: [200]
      extract: { external_account_id: ".id", external_login: ".login", account_type: ".type" }
      timeout: 5s
  contributions: { env: { TEST_TOKEN: "{{.Token}}" } }
`, id, id, displayName)
}

// ---------------------------------------------------------------------------
// Shared constants / helpers
// ---------------------------------------------------------------------------

const dbTestInterval = 50 * time.Millisecond

// waitDB blocks up to timeout for a signal on ch.
func waitDB(ch <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// ---------------------------------------------------------------------------
// Test 1 — initial poll loads manifests and tags Source == "db"
// ---------------------------------------------------------------------------

func TestDBRegistryInitialPollLoads(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha"), Enabled: true, Revision: 1}
	m2 := domain.PluginManifest{ID: "beta", YAML: dbTestManifest("beta", "Beta"), Enabled: true, Revision: 1}

	lister := newFakeLister(m1, m2)
	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}

	for _, r := range list {
		if r.Source != "db" {
			t.Errorf("expected Source=db, got %q for id=%s", r.Source, r.Manifest.Spec.ID)
		}
	}

	for _, id := range []string{"alpha", "beta"} {
		if _, ok := reg.Get(id); !ok {
			t.Errorf("expected %s to be present", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2 — parse error keeps prior; valid new row appears
// ---------------------------------------------------------------------------

func TestDBRegistryParseErrorKeepsPrior(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha v1"), Enabled: true, Revision: 1}
	lister := newFakeLister(m1)

	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Confirm initial state.
	r, ok := reg.Get("alpha")
	if !ok {
		t.Fatal("alpha not found after initial poll")
	}
	if r.Manifest.Spec.DisplayName != "Alpha v1" {
		t.Fatalf("unexpected initial display_name: %q", r.Manifest.Spec.DisplayName)
	}

	sub := reg.Subscribe()

	// Update: alpha at revision 2 with malformed YAML, plus a new valid beta.
	badAlpha := domain.PluginManifest{ID: "alpha", YAML: "not: : yaml: :::", Enabled: true, Revision: 2}
	goodBeta := domain.PluginManifest{ID: "beta", YAML: dbTestManifest("beta", "Beta v1"), Enabled: true, Revision: 1}
	lister.set(badAlpha, goodBeta)

	if !waitDB(sub, 2*time.Second) {
		t.Fatal("timed out waiting for change notification after parse-error update")
	}

	// alpha must still resolve to its prior valid manifest.
	rAlpha, ok := reg.Get("alpha")
	if !ok {
		t.Fatal("alpha disappeared after malformed update")
	}
	if rAlpha.Manifest.Spec.DisplayName != "Alpha v1" {
		t.Errorf("alpha display_name = %q, want %q (should keep prior)", rAlpha.Manifest.Spec.DisplayName, "Alpha v1")
	}

	// beta must appear as a new valid entry.
	rBeta, ok := reg.Get("beta")
	if !ok {
		t.Fatal("beta not found after update")
	}
	if rBeta.Manifest.Spec.DisplayName != "Beta v1" {
		t.Errorf("beta display_name = %q, want %q", rBeta.Manifest.Spec.DisplayName, "Beta v1")
	}
}

// ---------------------------------------------------------------------------
// Test 3 — change detection fires subscriber and updates Get
// ---------------------------------------------------------------------------

func TestDBRegistryChangeDetection(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha v1"), Enabled: true, Revision: 1}
	lister := newFakeLister(m1)

	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Subscribe after Run; initial poll has already completed.
	sub := reg.Subscribe()

	// Modify content + bump revision.
	updated := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha v2"), Enabled: true, Revision: 2}
	lister.set(updated)

	if !waitDB(sub, 2*time.Second) {
		t.Fatal("timed out waiting for change notification")
	}

	r, ok := reg.Get("alpha")
	if !ok {
		t.Fatal("alpha not found after update")
	}
	if r.Manifest.Spec.DisplayName != "Alpha v2" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "Alpha v2")
	}
}

// ---------------------------------------------------------------------------
// Test 4 — no notification on identical snapshot
// ---------------------------------------------------------------------------

func TestDBRegistryNoOpOnIdenticalSnapshot(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha"), Enabled: true, Revision: 1}
	lister := newFakeLister(m1)

	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Subscribe after Run (no initial broadcast expected).
	sub := reg.Subscribe()

	// Drain any stray notification within a short window.
	select {
	case <-sub:
	case <-time.After(30 * time.Millisecond):
	}

	// Let several poll cycles elapse without changing anything.
	time.Sleep(4 * dbTestInterval)

	// Assert no notification arrives in a 300ms window.
	select {
	case <-sub:
		t.Error("unexpected notification: identical snapshot should not broadcast")
	case <-time.After(300 * time.Millisecond):
		// success — no spurious notification
	}
}

// ---------------------------------------------------------------------------
// Test 5 — store error tolerated; state retained; polls resume after clear
// ---------------------------------------------------------------------------

func TestDBRegistryStoreErrorTolerated(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha"), Enabled: true, Revision: 1}
	lister := newFakeLister(m1)

	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify initial state.
	if _, ok := reg.Get("alpha"); !ok {
		t.Fatal("alpha not found after initial poll")
	}

	// Inject an error.
	lister.setErr(errors.New("db connection lost"))

	// Let several poll cycles run.
	time.Sleep(4 * dbTestInterval)

	// Prior state must still be available.
	r, ok := reg.Get("alpha")
	if !ok {
		t.Fatal("alpha disappeared after store error")
	}
	if r.Manifest.Spec.DisplayName != "Alpha" {
		t.Errorf("display_name changed under error: %q", r.Manifest.Spec.DisplayName)
	}

	// Subscribe, then clear error + mutate to verify polls resume.
	sub := reg.Subscribe()
	lister.setErr(nil)
	updated := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha Resumed"), Enabled: true, Revision: 2}
	lister.set(updated)

	if !waitDB(sub, 2*time.Second) {
		t.Fatal("timed out: polls did not resume after error cleared")
	}

	r, ok = reg.Get("alpha")
	if !ok {
		t.Fatal("alpha not found after resuming polls")
	}
	if r.Manifest.Spec.DisplayName != "Alpha Resumed" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "Alpha Resumed")
	}
}

// ---------------------------------------------------------------------------
// Test 6 — empty result clears the registry and notifies subscribers
// ---------------------------------------------------------------------------

func TestDBRegistryEmptyResultClears(t *testing.T) {
	m1 := domain.PluginManifest{ID: "alpha", YAML: dbTestManifest("alpha", "Alpha"), Enabled: true, Revision: 1}
	lister := newFakeLister(m1)

	reg := NewDBRegistry(lister, dbTestInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if list := reg.List(); len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}

	sub := reg.Subscribe()

	// Clear all manifests.
	lister.set()

	if !waitDB(sub, 2*time.Second) {
		t.Fatal("timed out waiting for clear notification")
	}

	if list := reg.List(); len(list) != 0 {
		t.Errorf("expected empty list after clear, got %d entries", len(list))
	}
}
