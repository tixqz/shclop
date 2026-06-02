package plugins

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/mipopov/shclop/internal/k8s"
)

// crdTestSetup creates the fake dynamic client, factory, informer, and registry
// for use in each test.
//
// We must NOT pass k8s.Scheme (which has typed objects) to the fake dynamic client.
// The dynamic informer expects to work with *unstructured.Unstructured objects.
// NewSimpleDynamicClient internally builds an "unstructured scheme" where all
// known GVKs are mapped to *unstructured.Unstructured — that is what we replicate
// here by building a plain scheme and registering the unstructured GVKs.
func crdTestSetup(t *testing.T, logger *slog.Logger) (
	client *fakedynamic.FakeDynamicClient,
	reg *CRDRegistry,
	ctx context.Context,
	cancel context.CancelFunc,
) {
	t.Helper()
	gvr := k8s.IntegrationPluginGVR

	// Build an all-unstructured scheme so the fake tracker returns *unstructured.Unstructured.
	unstructuredScheme := runtime.NewScheme()
	sgv := k8s.SchemeGroupVersion
	unstructuredScheme.AddKnownTypeWithName(sgv.WithKind("IntegrationPlugin"), &unstructured.Unstructured{})
	unstructuredScheme.AddKnownTypeWithName(sgv.WithKind("IntegrationPluginList"), &unstructured.UnstructuredList{})

	client = fakedynamic.NewSimpleDynamicClientWithCustomListKinds(unstructuredScheme, map[runtimeschema.GroupVersionResource]string{
		gvr: "IntegrationPluginList",
	})
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 0, metav1.NamespaceAll, nil)
	informer := factory.ForResource(gvr).Informer()

	ctx, cancel = context.WithCancel(context.Background())

	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	reg = NewCRDRegistry(informer, logger)

	factory.Start(ctx.Done())
	go reg.Run(ctx) //nolint:errcheck

	// Wait for the informer cache to sync before returning so tests don't race
	// against handler registration.
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("crdTestSetup: informer failed to sync")
	}

	return client, reg, ctx, cancel
}

// buildIntegrationPlugin returns a valid *k8s.IntegrationPlugin Go object.
// displayName allows customisation for update tests.
func buildIntegrationPlugin(name, displayName string) *k8s.IntegrationPlugin {
	return &k8s.IntegrationPlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "shclop.io/v1alpha1",
			Kind:       "IntegrationPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: k8s.IntegrationPluginSpec{
			ID:          "github",
			DisplayName: displayName,
			Description: "Connect a GitHub PAT to expose gh CLI and MCP tools in agent runtime",
			PluginKind:  "http_template",
			ScopesSupported: []string{"user"},
			Auth: k8s.AuthSpec{
				Type: "pat_token",
				Fields: []k8s.FormField{
					{
						Name:        "token",
						Label:       "Personal Access Token",
						Secret:      true,
						Placeholder: "ghp_...",
						HelpURL:     "https://github.com/settings/tokens",
					},
				},
			},
			Validate: k8s.ValidateSpec{
				HTTP: &k8s.HTTPValidate{
					Method: "GET",
					URL:    "https://api.github.com/user",
					Headers: map[string]string{
						"Authorization": "Bearer {{.Token}}",
						"User-Agent":    "shclop/1.0",
					},
					SuccessStatus: []int{200},
					Extract: k8s.ExtractSpec{
						ExternalAccountID: ".id",
						ExternalLogin:     ".login",
						AccountType:       ".type",
					},
					Timeout: "5s",
				},
			},
			Contributions: k8s.Contributions{
				Env: map[string]string{"GITHUB_TOKEN": "{{.Token}}"},
			},
		},
	}
}

// toUnstructured converts a typed IntegrationPlugin to *unstructured.Unstructured.
func toUnstructured(t *testing.T, ip *k8s.IntegrationPlugin) *unstructured.Unstructured {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ip)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: obj}
	u.SetGroupVersionKind(k8s.SchemeGroupVersion.WithKind("IntegrationPlugin"))
	return u
}

// waitGet polls reg.Get(id) every 10ms until found or timeout.
func waitGet(reg *CRDRegistry, id string, timeout time.Duration) (Resolved, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r, ok := reg.Get(id); ok {
			return r, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return Resolved{}, false
}

// waitGone polls reg.Get(id) every 10ms until absent or timeout.
func waitGone(reg *CRDRegistry, id string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, ok := reg.Get(id); !ok {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// drainSubscriber drains all pending notifications from ch (non-blocking).
func drainSubscriber(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func TestCRDRegistryAdd(t *testing.T) {
	client, reg, _, cancel := crdTestSetup(t, nil)
	defer cancel()

	ch := reg.Subscribe()

	gvr := k8s.IntegrationPluginGVR
	u := toUnstructured(t, buildIntegrationPlugin("github", "GitHub"))
	if _, err := client.Resource(gvr).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for subscriber notification after Create")
	}

	r, ok := reg.Get("github")
	if !ok {
		t.Fatal("Get(github) returned false after Create")
	}
	if r.Manifest.Spec.ID != "github" {
		t.Errorf("manifest id = %q, want %q", r.Manifest.Spec.ID, "github")
	}
	if r.Source != "crd" {
		t.Errorf("source = %q, want %q", r.Source, "crd")
	}
}

func TestCRDRegistryUpdate(t *testing.T) {
	client, reg, _, cancel := crdTestSetup(t, nil)
	defer cancel()

	gvr := k8s.IntegrationPluginGVR

	// Create initial object.
	u := toUnstructured(t, buildIntegrationPlugin("github", "GitHub"))
	if _, err := client.Resource(gvr).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait for initial add to land before subscribing for the update.
	if _, ok := waitGet(reg, "github", 2*time.Second); !ok {
		t.Fatal("timed out waiting for initial Create to be indexed")
	}

	ch := reg.Subscribe()
	drainSubscriber(ch)

	// Update with a new display_name.
	updated := buildIntegrationPlugin("github", "GitHub Updated")
	uUpdated := toUnstructured(t, updated)
	if _, err := client.Resource(gvr).Update(context.Background(), uUpdated, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for subscriber notification after Update")
	}

	r, ok := reg.Get("github")
	if !ok {
		t.Fatal("Get(github) returned false after Update")
	}
	if r.Manifest.Spec.DisplayName != "GitHub Updated" {
		t.Errorf("display_name = %q, want %q", r.Manifest.Spec.DisplayName, "GitHub Updated")
	}
}

func TestCRDRegistryDelete(t *testing.T) {
	client, reg, _, cancel := crdTestSetup(t, nil)
	defer cancel()

	gvr := k8s.IntegrationPluginGVR

	u := toUnstructured(t, buildIntegrationPlugin("github", "GitHub"))
	if _, err := client.Resource(gvr).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait for it to appear.
	if _, ok := waitGet(reg, "github", 2*time.Second); !ok {
		t.Fatal("timed out waiting for Create to be indexed")
	}

	ch := reg.Subscribe()
	drainSubscriber(ch)

	if err := client.Resource(gvr).Delete(context.Background(), "github", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if !waitNotify(ch, 2*time.Second) {
		t.Fatal("timed out waiting for subscriber notification after Delete")
	}

	if _, ok := reg.Get("github"); ok {
		t.Error("Get(github) returned true after Delete; expected it to be gone")
	}
}

func TestCRDRegistryMalformedSpec(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, reg, _, cancel := crdTestSetup(t, logger)
	defer cancel()

	gvr := k8s.IntegrationPluginGVR

	// Build a CR with spec.kind = "invalid" so Manifest.Validate() rejects it.
	bad := buildIntegrationPlugin("bad-plugin", "Bad Plugin")
	bad.Spec.PluginKind = "invalid"
	u := toUnstructured(t, bad)

	if _, err := client.Resource(gvr).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait for the WARN log line that mentions the CR name. The AddFunc handler runs
	// asynchronously so we poll until we see it or timeout.
	deadline := time.Now().Add(2 * time.Second)
	var logOutput string
	for time.Now().Before(deadline) {
		logOutput = buf.String()
		if bytes.Contains([]byte(logOutput), []byte("bad-plugin")) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// WARN log should mention the CR name.
	if !bytes.Contains([]byte(logOutput), []byte("bad-plugin")) {
		t.Errorf("WARN log does not contain CR name %q; got: %s", "bad-plugin", logOutput)
	}

	// No entry should be in resolved.
	if _, ok := reg.Get("bad-plugin"); ok {
		t.Error("Get(bad-plugin) returned true for a malformed spec; expected nothing stored")
	}
	// Also confirm by manifest id — the bad-plugin spec has id="github", should NOT be stored.
	if _, ok := reg.Get("github"); ok {
		t.Error("malformed plugin stored under id=github despite validation failure")
	}
}

// TestCRDRegistryGoneAfterDeleteConfirmPolling is a belt-and-suspenders check
// that waitGone works as expected: after Delete, Get eventually returns false.
func TestCRDRegistryGoneAfterDeleteConfirmPolling(t *testing.T) {
	client, reg, _, cancel := crdTestSetup(t, nil)
	defer cancel()

	gvr := k8s.IntegrationPluginGVR
	u := toUnstructured(t, buildIntegrationPlugin("github", "GitHub"))
	if _, err := client.Resource(gvr).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, ok := waitGet(reg, "github", 2*time.Second); !ok {
		t.Fatal("timed out waiting for initial Create")
	}
	if err := client.Resource(gvr).Delete(context.Background(), "github", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !waitGone(reg, "github", 2*time.Second) {
		t.Error("Get(github) still returned true 2s after Delete")
	}
}
