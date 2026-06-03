package identity

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/store"
)

func newTestLogger() *slog.Logger {
	return slog.Default()
}

func cfgFor(name string, idp *testIDP) OIDCProviderConfig {
	return OIDCProviderConfig{
		Name:         name,
		Issuer:       idp.server.URL,
		ClientID:     "client-id",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/callback",
	}
}

func TestRegistry_MaterializeAllReady(t *testing.T) {
	idp1 := newTestIDP(t)
	idp2 := newTestIDP(t)

	cfg1 := cfgFor("p1", idp1)
	cfg2 := cfgFor("p2", idp2)

	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfg1, cfg2}, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	for _, name := range []string{"p1", "p2"} {
		e, ok := reg.Get(name)
		if !ok {
			t.Fatalf("Get(%q) not found", name)
		}
		if e.Status != StatusReady {
			t.Errorf("Get(%q).Status = %q, want ready", name, e.Status)
		}
		if e.Provider == nil {
			t.Errorf("Get(%q).Provider is nil", name)
		}
	}
}

func TestRegistry_MaterializeOneDegraded(t *testing.T) {
	idp1 := newTestIDP(t)

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(badSrv.Close)

	cfgGood := cfgFor("healthy", idp1)
	cfgBad := OIDCProviderConfig{
		Name:         "broken",
		Issuer:       badSrv.URL,
		ClientID:     "client-id",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/callback",
	}

	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfgGood, cfgBad}, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	healthy, ok := reg.Get("healthy")
	if !ok {
		t.Fatal("Get(healthy) not found")
	}
	if healthy.Status != StatusReady {
		t.Errorf("healthy.Status = %q, want ready", healthy.Status)
	}

	broken, ok := reg.Get("broken")
	if !ok {
		t.Fatal("Get(broken) not found")
	}
	if broken.Status != StatusDegraded {
		t.Errorf("broken.Status = %q, want degraded", broken.Status)
	}
	if broken.LastErr == "" {
		t.Error("broken.LastErr should be non-empty")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), nil, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}
	e, ok := reg.Get("nonexistent")
	if ok || e != nil {
		t.Errorf("Get(nonexistent) = %v, %v; want nil, false", e, ok)
	}
}

func TestRegistry_ListSortedByName(t *testing.T) {
	idp1 := newTestIDP(t)
	idp2 := newTestIDP(t)

	cfgZ := cfgFor("zebra", idp1)
	cfgA := cfgFor("alpha", idp2)

	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfgZ, cfgA}, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	if list[0].Config.Name != "alpha" {
		t.Errorf("List()[0].Name = %q, want alpha", list[0].Config.Name)
	}
	if list[1].Config.Name != "zebra" {
		t.Errorf("List()[1].Name = %q, want zebra", list[1].Config.Name)
	}
}

func TestRegistry_OverlayEnabledFlag(t *testing.T) {
	idp1 := newTestIDP(t)
	idp2 := newTestIDP(t)

	cfgFoo := cfgFor("foo", idp1)
	cfgBar := cfgFor("bar", idp2)

	st := store.NewMemory()
	if err := st.SetAppSetting(t.Context(), "auth.idp.foo.enabled", "false"); err != nil {
		t.Fatalf("SetAppSetting: %v", err)
	}

	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfgFoo, cfgBar}, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	foo, _ := reg.Get("foo")
	if foo.Enabled {
		t.Error("foo.Enabled = true, want false")
	}

	bar, _ := reg.Get("bar")
	if !bar.Enabled {
		t.Error("bar.Enabled = false, want true (default missing)")
	}
}

func TestRegistry_Mode(t *testing.T) {
	st := store.NewMemory()
	if err := st.SetAppSetting(t.Context(), "auth.mode", "sso"); err != nil {
		t.Fatalf("SetAppSetting: %v", err)
	}

	reg, err := NewIdPRegistry(t.Context(), nil, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}
	if reg.Mode() != domain.AuthModeSSO {
		t.Errorf("Mode() = %q, want sso", reg.Mode())
	}

	// Empty string → local
	st2 := store.NewMemory()
	if err := st2.SetAppSetting(t.Context(), "auth.mode", ""); err != nil {
		t.Fatalf("SetAppSetting: %v", err)
	}
	reg2, err := NewIdPRegistry(t.Context(), nil, st2, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}
	if reg2.Mode() != domain.AuthModeLocal {
		t.Errorf("Mode() = %q, want local", reg2.Mode())
	}
}

func TestRegistry_Reload_UpdatesOverlay(t *testing.T) {
	idp1 := newTestIDP(t)
	cfgFoo := cfgFor("foo", idp1)

	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfgFoo}, st, 0, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	sub := reg.Subscribe()

	before, _ := reg.Get("foo")
	if !before.Enabled {
		t.Error("foo.Enabled should default to true before change")
	}

	if err := st.SetAppSetting(t.Context(), "auth.idp.foo.enabled", "false"); err != nil {
		t.Fatalf("SetAppSetting: %v", err)
	}
	if err := reg.Reload(t.Context()); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	after, _ := reg.Get("foo")
	if after.Enabled {
		t.Error("foo.Enabled = true after Reload, want false")
	}

	select {
	case <-sub:
	case <-time.After(200 * time.Millisecond):
		t.Error("Subscribe channel did not receive notification after Reload")
	}
}

func TestRegistry_DegradedRetry(t *testing.T) {
	var reqCount atomic.Int32

	muxPtr := http.NewServeMux()
	var srvURL string

	muxPtr.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		if n == 1 {
			http.Error(w, "not ready", http.StatusInternalServerError)
			return
		}
		doc := map[string]any{
			"issuer":                                srvURL,
			"authorization_endpoint":                srvURL + "/auth",
			"token_endpoint":                        srvURL + "/token",
			"jwks_uri":                              srvURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})
	muxPtr.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})

	srv := httptest.NewServer(muxPtr)
	t.Cleanup(srv.Close)
	srvURL = srv.URL

	cfg := OIDCProviderConfig{
		Name:         "flaky",
		Issuer:       srvURL,
		ClientID:     "client-id",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/callback",
	}

	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), []OIDCProviderConfig{cfg}, st, 50*time.Millisecond, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	initial, _ := reg.Get("flaky")
	if initial.Status != StatusDegraded {
		t.Fatalf("expected degraded after first 500, got %q", initial.Status)
	}

	sub := reg.Subscribe()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	select {
	case <-sub:
	case <-time.After(800 * time.Millisecond):
		t.Error("Subscribe did not fire after degraded retry success")
	}

	recovered, _ := reg.Get("flaky")
	if recovered.Status != StatusReady {
		t.Errorf("expected ready after retry, got %q", recovered.Status)
	}
}

func TestRegistry_RunIdempotent(t *testing.T) {
	st := store.NewMemory()
	reg, err := NewIdPRegistry(t.Context(), nil, st, 50*time.Millisecond, newTestLogger())
	if err != nil {
		t.Fatalf("NewIdPRegistry: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run (first): %v", err)
	}
	if err := reg.Run(ctx); err != nil {
		t.Fatalf("Run (second): %v", err)
	}
}
