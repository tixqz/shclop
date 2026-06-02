package plugins

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/mipopov/shclop/internal/k8s"
)

var resolverLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func makeMCPServerUnstructured(name, namespace, image string, port int64, serviceURL string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   k8s.GroupName,
		Version: k8s.GroupVersion,
		Kind:    "MCPServer",
	})
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetResourceVersion("1")
	_ = unstructured.SetNestedField(u.Object, image, "spec", "image")
	_ = unstructured.SetNestedField(u.Object, port, "spec", "port")
	if serviceURL != "" {
		_ = unstructured.SetNestedField(u.Object, serviceURL, "status", "serviceURL")
	}
	return u
}

func newResolverInformer(t *testing.T, ns string, objs ...*unstructured.Unstructured) (cache.SharedIndexInformer, context.CancelFunc) {
	t.Helper()

	dynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		k8s.Scheme,
		map[schema.GroupVersionResource]string{
			k8s.MCPServerGVR: "MCPServerList",
		},
	)

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, ns, nil)
	informer := factory.ForResource(k8s.MCPServerGVR).Informer()

	ctx, cancel := context.WithCancel(context.Background())
	factory.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	// Directly populate the indexer so tests don't need a live API server.
	for _, obj := range objs {
		if err := informer.GetIndexer().Add(obj); err != nil {
			t.Fatalf("add to indexer: %v", err)
		}
	}

	return informer, cancel
}

func TestResolveExternalMCP(t *testing.T) {
	informer, stop := newResolverInformer(t, "test-ns")
	defer stop()

	resolver := NewMCPResolver(informer, "test-ns", resolverLogger)

	refs := []MCPRef{
		{
			Name: "my-mcp",
			External: &ExternalMCP{
				URL:        "{{.Fields.host}}/mcp",
				AuthHeader: "Bearer {{.Token}}",
			},
		},
	}
	vars := TemplateContext{
		Token:  "tok-abc",
		Fields: map[string]string{"host": "https://mcp.example.com"},
	}

	results, err := resolver.Resolve(context.Background(), refs, vars)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.Name != "my-mcp" {
		t.Errorf("name: want %q, got %q", "my-mcp", got.Name)
	}
	if got.URL != "https://mcp.example.com/mcp" {
		t.Errorf("url: want %q, got %q", "https://mcp.example.com/mcp", got.URL)
	}
	if got.Headers["Authorization"] != "Bearer tok-abc" {
		t.Errorf("Authorization header: want %q, got %q", "Bearer tok-abc", got.Headers["Authorization"])
	}
}

func TestResolveClusterRefPresent(t *testing.T) {
	const ns = "test-ns"
	const crName = "github-mcp"
	const expectedURL = "http://mcp-server.example.svc:8080"

	mcpObj := makeMCPServerUnstructured(crName, ns, "ghcr.io/test/mcp:1.0", 8080, expectedURL)
	informer, stop := newResolverInformer(t, ns, mcpObj)
	defer stop()

	resolver := NewMCPResolver(informer, ns, resolverLogger)

	refs := []MCPRef{
		{Name: "gh-tools", Ref: crName},
	}

	results, err := resolver.Resolve(context.Background(), refs, TemplateContext{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != expectedURL {
		t.Errorf("url: want %q, got %q", expectedURL, results[0].URL)
	}
}

func TestResolveClusterRefAbsent(t *testing.T) {
	informer, stop := newResolverInformer(t, "test-ns")
	defer stop()

	resolver := NewMCPResolver(informer, "test-ns", resolverLogger)

	refs := []MCPRef{
		{Name: "missing", Ref: "nonexistent-mcp"},
	}

	_, err := resolver.Resolve(context.Background(), refs, TemplateContext{})
	if err == nil {
		t.Fatal("expected error for absent MCPServer ref, got nil")
	}
	if !strings.Contains(err.Error(), "mcpserver_not_found") {
		t.Errorf("expected 'mcpserver_not_found' in error, got: %v", err)
	}
}

func TestResolveClusterRefDerivedURL(t *testing.T) {
	const ns = "runtime-ns"
	const crName = "analytics"
	const port = int64(9090)

	// No status.serviceURL set — controller hasn't updated status yet.
	mcpObj := makeMCPServerUnstructured(crName, ns, "ghcr.io/analytics/mcp:latest", port, "")
	informer, stop := newResolverInformer(t, ns, mcpObj)
	defer stop()

	resolver := NewMCPResolver(informer, ns, resolverLogger)

	refs := []MCPRef{
		{Name: "analytics-tools", Ref: crName},
	}

	results, err := resolver.Resolve(context.Background(), refs, TemplateContext{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	wantURL := "http://mcp-analytics.runtime-ns.svc.cluster.local:9090"
	if results[0].URL != wantURL {
		t.Errorf("derived url: want %q, got %q", wantURL, results[0].URL)
	}
}

// Verify metav1 import is used (it's needed for GetOptions in the indexer test helpers).
var _ = metav1.GetOptions{}
