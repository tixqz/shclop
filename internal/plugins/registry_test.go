package plugins

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRegistry is an in-memory Registry for testing.
type fakeRegistry struct {
	mu       sync.Mutex
	items    map[string]Manifest
	src      string
	notifyCh chan struct{}
}

func newFake(src string) *fakeRegistry {
	return &fakeRegistry{
		src:      src,
		items:    make(map[string]Manifest),
		notifyCh: make(chan struct{}, 1),
	}
}

func (f *fakeRegistry) Set(id string, m Manifest) {
	f.mu.Lock()
	f.items[id] = m
	f.mu.Unlock()
	select {
	case f.notifyCh <- struct{}{}:
	default:
	}
}

func (f *fakeRegistry) Remove(id string) {
	f.mu.Lock()
	delete(f.items, id)
	f.mu.Unlock()
	select {
	case f.notifyCh <- struct{}{}:
	default:
	}
}

func (f *fakeRegistry) Get(id string) (Resolved, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok {
		return Resolved{}, false
	}
	return Resolved{Manifest: m, Source: f.src}, true
}

func (f *fakeRegistry) List() []Resolved {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Resolved, 0, len(f.items))
	for _, m := range f.items {
		out = append(out, Resolved{Manifest: m, Source: f.src})
	}
	return out
}

func (f *fakeRegistry) Subscribe() <-chan struct{} {
	return f.notifyCh
}

func (f *fakeRegistry) Run(_ context.Context) error { return nil }

// minimalManifest builds a valid Manifest with the given id and a distinguishing display name.
func minimalManifest(id, displayName string) Manifest {
	return Manifest{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   Metadata{Name: id},
		Spec: ManifestSpec{
			ID:              id,
			DisplayName:     displayName,
			PluginKind:      "http_template",
			ScopesSupported: []string{"user"},
			Auth: AuthSpec{
				Type:   "pat_token",
				Fields: []FormField{{Name: "token", Label: "Token"}},
			},
			Validate: ValidateSpec{
				HTTP: &HTTPValidate{
					Method:        "GET",
					URL:           "https://example.com",
					SuccessStatus: []int{200},
				},
			},
			Contributions: Contributions{
				Env: map[string]string{},
			},
		},
	}
}

// TestPrecedence: highest-precedence source wins; lower-precedence entries fall through.
func TestPrecedence(t *testing.T) {
	crd := newFake("crd")
	db := newFake("db")
	file := newFake("file")

	crd.Set("github", minimalManifest("github", "GitHub CRD"))
	db.Set("github", minimalManifest("github", "GitHub DB"))
	file.Set("gitlab", minimalManifest("gitlab", "GitLab"))

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	merged := NewMerged(logger, crd, db, file)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, ok := merged.Get("github")
	if !ok {
		t.Fatal("expected github to be present")
	}
	if got.Source != "crd" {
		t.Errorf("expected source=crd, got %q", got.Source)
	}

	got2, ok := merged.Get("gitlab")
	if !ok {
		t.Fatal("expected gitlab to be present")
	}
	if got2.Source != "file" {
		t.Errorf("expected source=file, got %q", got2.Source)
	}
}

// TestFallThroughOnRemoval: after higher-precedence entry is removed, lower takes over.
func TestFallThroughOnRemoval(t *testing.T) {
	crd := newFake("crd")
	db := newFake("db")

	crd.Set("github", minimalManifest("github", "GitHub CRD"))
	db.Set("github", minimalManifest("github", "GitHub DB"))

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	merged := NewMerged(logger, crd, db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify CRD wins initially.
	got, ok := merged.Get("github")
	if !ok || got.Source != "crd" {
		t.Fatalf("expected crd to win initially, got ok=%v source=%q", ok, got.Source)
	}

	// Subscribe so we can wait for the rebuild.
	sub := merged.Subscribe()

	crd.Remove("github")

	select {
	case <-sub:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for change notification after removal")
	}

	got, ok = merged.Get("github")
	if !ok {
		t.Fatal("expected github to still be present via db")
	}
	if got.Source != "db" {
		t.Errorf("expected source=db after crd removal, got %q", got.Source)
	}
}

// TestSubscribeReceivesOnChange: subscriber channel receives when a source is mutated.
func TestSubscribeReceivesOnChange(t *testing.T) {
	src := newFake("file")

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	merged := NewMerged(logger, src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	sub := merged.Subscribe()

	src.Set("myplugin", minimalManifest("myplugin", "My Plugin"))

	select {
	case <-sub:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out: subscriber did not receive notification within 200ms")
	}
}

// TestSubscribeNonBlocking: multiple rapid updates don't deadlock when no one reads the channel.
func TestSubscribeNonBlocking(t *testing.T) {
	src := newFake("file")

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	merged := NewMerged(logger, src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Subscribe but never read.
	_ = merged.Subscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 20 {
			src.Set("p"+string(rune('a'+i)), minimalManifest("p"+string(rune('a'+i)), "Plugin"))
		}
	}()

	select {
	case <-done:
		// success: no deadlock
	case <-time.After(500 * time.Millisecond):
		t.Fatal("deadlock: rapid updates blocked source with unread subscriber channel")
	}
}

// TestShadowLog: shadowed entries produce a WARN log; same event is only logged once.
func TestShadowLog(t *testing.T) {
	crd := newFake("crd")
	file := newFake("file")

	// Same id, different display names → different spec hashes.
	crd.Set("github", minimalManifest("github", "GitHub from CRD"))
	file.Set("github", minimalManifest("github", "GitHub from File"))

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	merged := NewMerged(logger, crd, file)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Allow rebuild goroutines to settle (Run does initial rebuild synchronously,
	// but we also want to confirm the log was written).
	time.Sleep(20 * time.Millisecond)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "plugin shadowed") {
		t.Errorf("expected 'plugin shadowed' in log, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "github") {
		t.Errorf("expected id 'github' in shadow log, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "crd") {
		t.Errorf("expected winning source 'crd' in shadow log, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "file") {
		t.Errorf("expected shadowed source 'file' in shadow log, got:\n%s", logOutput)
	}

	// Record how many times the message appeared.
	firstCount := strings.Count(logOutput, "plugin shadowed")

	// Subscribe so we can wait for rebuild.
	sub := merged.Subscribe()

	// Trigger another rebuild with the same shadow (no content change).
	crd.Set("other", minimalManifest("other", "Other"))

	select {
	case <-sub:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for change notification")
	}

	// Wait for the log to flush.
	time.Sleep(20 * time.Millisecond)

	secondCount := strings.Count(buf.String(), "plugin shadowed")
	// The same shadow event should not be logged again.
	if secondCount != firstCount {
		t.Errorf("shadow event logged again on rebuild with same content: count went %d→%d", firstCount, secondCount)
	}
}

// TestRunIdempotent: calling Run twice should not panic or spawn double watchers.
func TestRunIdempotent(t *testing.T) {
	src := newFake("file")
	src.Set("p1", minimalManifest("p1", "Plugin One"))

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	merged := NewMerged(logger, src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := merged.Run(ctx); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := merged.Run(ctx); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// Verify the registry is still consistent.
	got, ok := merged.Get("p1")
	if !ok {
		t.Fatal("expected p1 to be present after double Run")
	}
	if got.Source != "file" {
		t.Errorf("expected source=file, got %q", got.Source)
	}
}
