package plugins

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// validManifestYAML returns a valid plugin manifest YAML for the given id and displayName.
func validManifestYAML(id, displayName string) string {
	return `apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: ` + id + ` }
spec:
  id: ` + id + `
  display_name: ` + displayName + `
  kind: http_template
  scopes_supported: [user]
  auth:
    type: pat_token
    fields:
      - { name: token, label: Token, secret: true }
  validate:
    http:
      method: GET
      url: "https://api.example.com/me"
      headers: { Authorization: "Bearer {{.Token}}" }
      success_status: [200]
      extract: { external_account_id: ".id", external_login: ".login", account_type: ".type" }
      timeout: 5s
  contributions:
    env: { TEST_TOKEN: "{{.Token}}" }
`
}

// writeYAML writes content to path/filename in the given dir.
func writeYAML(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}
	return path
}

// waitNotify waits up to timeout for a signal on ch.
// Returns true if received, false on timeout.
func waitNotify(ch <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestInitialScanLoadsFiles verifies that two valid yaml files are loaded on startup.
func TestInitialScanLoadsFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "plugin1.yaml", validManifestYAML("plugin1", "Plugin 1"))
	writeYAML(t, dir, "plugin2.yaml", validManifestYAML("plugin2", "Plugin 2"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}

	if _, ok := reg.Get("plugin1"); !ok {
		t.Error("plugin1 not found")
	}
	if _, ok := reg.Get("plugin2"); !ok {
		t.Error("plugin2 not found")
	}
}

// TestCreateNewFileFiresNotify verifies that adding a new file triggers a subscriber notification.
func TestCreateNewFileFiresNotify(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ch := reg.Subscribe()

	writeYAML(t, dir, "newplugin.yaml", validManifestYAML("newplugin", "New Plugin"))

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for subscriber notification after file creation")
	}

	r, ok := reg.Get("newplugin")
	if !ok {
		t.Fatal("newplugin not found after notify")
	}
	if r.Manifest.Spec.DisplayName != "New Plugin" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "New Plugin")
	}
}

// TestModifyFileFiresNotify verifies that overwriting a file triggers a notification
// and the updated manifest is reflected.
func TestModifyFileFiresNotify(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "mod.yaml", validManifestYAML("mod", "Original Name"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ch := reg.Subscribe()

	// Overwrite with new display_name.
	writeYAML(t, dir, "mod.yaml", validManifestYAML("mod", "Updated Name"))

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for notification after file modification")
	}

	r, ok := reg.Get("mod")
	if !ok {
		t.Fatal("mod not found after modify")
	}
	if r.Manifest.Spec.DisplayName != "Updated Name" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "Updated Name")
	}
}

// TestDeleteFileFiresNotify verifies that removing a file triggers a notification
// and the entry is gone from the registry.
func TestDeleteFileFiresNotify(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "del.yaml", validManifestYAML("del", "Delete Me"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ch := reg.Subscribe()

	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove: %v", err)
	}

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for notification after file deletion")
	}

	if _, ok := reg.Get("del"); ok {
		t.Error("del still present after file deletion")
	}
}

// TestMalformedFileKeepsPrevious verifies that when a valid file is overwritten with
// invalid YAML the previous manifest is retained.
func TestMalformedFileKeepsPrevious(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "stable.yaml", validManifestYAML("stable", "Stable"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Confirm initial state.
	if _, ok := reg.Get("stable"); !ok {
		t.Fatal("stable not found after initial scan")
	}

	ch := reg.Subscribe()

	// Overwrite with malformed YAML.
	writeYAML(t, dir, "stable.yaml", "not: : yaml :::")

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for notification after malformed overwrite")
	}

	// The previous valid manifest should still be present.
	r, ok := reg.Get("stable")
	if !ok {
		t.Fatal("stable not found after malformed overwrite (expected keep-previous)")
	}
	if r.Manifest.Spec.DisplayName != "Stable" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "Stable")
	}
}

// TestMissingDirDoesNotError verifies that Run returns nil for a non-existent directory
// and List returns empty.
func TestMissingDirDoesNotError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry("/tmp/does-not-exist-shclop-test-xyz", newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run returned error for missing dir: %v", err)
	}

	list := reg.List()
	if len(list) != 0 {
		t.Errorf("expected empty list for missing dir, got %d", len(list))
	}
}

// TestSkipDotFiles verifies that files starting with "." are ignored.
func TestSkipDotFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a dot-file with otherwise valid content.
	writeYAML(t, dir, ".hidden.yaml", validManifestYAML("hidden", "Hidden"))
	// Also write one legitimate file so we know the scan ran.
	writeYAML(t, dir, "visible.yaml", validManifestYAML("visible", "Visible"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, ok := reg.Get("hidden"); ok {
		t.Error("dot-file was loaded but should have been skipped")
	}
	if _, ok := reg.Get("visible"); !ok {
		t.Error("visible.yaml was not loaded")
	}
}

// TestDebounce verifies that a burst of writes produces at most 2 broadcasts within 500ms.
func TestDebounce(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bounce.yaml", validManifestYAML("bounce", "Bounce"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewFileRegistry(dir, newTestLogger())
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ch := reg.Subscribe()

	// Burst of 5 writes in ~50ms.
	for i := range 5 {
		name := "bounce"
		if i%2 == 0 {
			name = "bounce"
		}
		writeYAML(t, dir, "bounce.yaml", validManifestYAML(name, "Bounce"))
		time.Sleep(10 * time.Millisecond)
	}

	// Count broadcasts within 500ms window.
	count := 0
	deadline := time.After(500 * time.Millisecond)
drain:
	for {
		select {
		case <-ch:
			count++
		case <-deadline:
			break drain
		}
	}

	// Debounce should coalesce the burst into at most 2 broadcasts.
	if count > 2 {
		t.Errorf("expected ≤2 broadcasts for burst of 5 writes, got %d", count)
	}
}
