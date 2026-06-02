package controllers

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/mipopov/shclop/internal/k8s"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func makeTestMCPServer(name, namespace, image string, port int32, replicas int32) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   k8s.GroupName,
		Version: k8s.GroupVersion,
		Kind:    "MCPServer",
	})
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetResourceVersion("1")
	if err := unstructured.SetNestedField(u.Object, image, "spec", "image"); err != nil {
		panic(err)
	}
	if err := unstructured.SetNestedField(u.Object, int64(port), "spec", "port"); err != nil {
		panic(err)
	}
	if replicas > 0 {
		if err := unstructured.SetNestedField(u.Object, int64(replicas), "spec", "replicas"); err != nil {
			panic(err)
		}
	}
	return u
}

// newTestSetup creates a fake k8s.Client, a dynamic informer for MCPServer,
// and starts the informer factory. Objects are added to the informer indexer
// directly so the fake dynamic client's List path is never exercised.
func newTestSetup(t *testing.T, ns string, objs ...*unstructured.Unstructured) (*k8s.Client, cache.SharedIndexInformer, context.CancelFunc) {
	t.Helper()

	// Empty dynamic fake client — no initial objects to avoid scheme-conversion
	// issues in the fake List handler.
	dynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		k8s.Scheme,
		map[schema.GroupVersionResource]string{
			k8s.MCPServerGVR: "MCPServerList",
		},
	)

	kubeClient := kubefake.NewClientset()

	client := &k8s.Client{
		Dynamic:    dynClient,
		Kubernetes: kubeClient,
		Namespace:  ns,
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, ns, nil)
	informer := factory.ForResource(k8s.MCPServerGVR).Informer()

	ctx, cancel := context.WithCancel(context.Background())
	factory.Start(ctx.Done())

	// Wait for the cache to sync (will succeed immediately with empty list).
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		cancel()
		t.Fatal("informer cache did not sync")
	}

	// Seed the indexer directly — bypasses the fake List/Watch round-trip.
	for _, obj := range objs {
		if err := informer.GetIndexer().Add(obj); err != nil {
			cancel()
			t.Fatalf("add to indexer: %v", err)
		}
	}

	return client, informer, cancel
}

// waitFor polls until fn returns true or timeout.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func TestReconcileCreatesDeploymentServiceNetworkPolicy(t *testing.T) {
	const ns = "test-ns"
	const crName = "my-mcp"
	const image = "ghcr.io/test/mcp:1.0"
	const port = int32(8080)

	cr := makeTestMCPServer(crName, ns, image, port, 1)
	client, informer, stop := newTestSetup(t, ns, cr)
	defer stop()

	ctrl := NewMCPServerController(client, informer, ns, discardLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Enqueue the CR key manually so the controller processes it.
	ctrl.queue.Add(ns + "/" + crName)

	go func() { _ = ctrl.Run(ctx, 1) }()

	kubeClient := client.Kubernetes

	waitFor(t, 3*time.Second, func() bool {
		_, err := kubeClient.AppsV1().Deployments(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
		return err == nil
	})

	waitFor(t, 3*time.Second, func() bool {
		_, err := kubeClient.CoreV1().Services(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
		return err == nil
	})

	waitFor(t, 3*time.Second, func() bool {
		_, err := kubeClient.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
		return err == nil
	})

	// Assert Deployment fields.
	deploy, err := kubeClient.AppsV1().Deployments(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if len(deploy.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("no containers in deployment")
	}
	container := deploy.Spec.Template.Spec.Containers[0]
	if container.Image != image {
		t.Errorf("container image: want %q, got %q", image, container.Image)
	}
	if len(container.Ports) == 0 || container.Ports[0].ContainerPort != port {
		t.Errorf("container port: want %d, got %v", port, container.Ports)
	}
	if deploy.Labels["shclop.io/mcp-server"] != crName {
		t.Errorf("deployment missing label shclop.io/mcp-server=%s", crName)
	}

	// Assert Service fields.
	svc, err := kubeClient.CoreV1().Services(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if svc.Spec.Selector["shclop.io/mcp-server"] != crName {
		t.Errorf("service selector missing shclop.io/mcp-server=%s", crName)
	}
	if len(svc.Spec.Ports) == 0 || svc.Spec.Ports[0].Port != port {
		t.Errorf("service port: want %d, got %v", port, svc.Spec.Ports)
	}

	// Assert NetworkPolicy ingress rule.
	np, err := kubeClient.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), "mcp-"+crName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get networkpolicy: %v", err)
	}
	if len(np.Spec.Ingress) == 0 {
		t.Fatal("networkpolicy has no ingress rules")
	}
	ingressFrom := np.Spec.Ingress[0].From
	if len(ingressFrom) == 0 {
		t.Fatal("networkpolicy ingress has no from peers")
	}
	if ingressFrom[0].PodSelector == nil {
		t.Fatal("networkpolicy ingress from has no podSelector")
	}
	if ingressFrom[0].PodSelector.MatchLabels["shclop.io/role"] != "agent-runtime" {
		t.Errorf("expected ingress from shclop.io/role=agent-runtime, got %v",
			ingressFrom[0].PodSelector.MatchLabels)
	}
}

func TestReconcileIdempotent(t *testing.T) {
	const ns = "test-ns"
	const crName = "idempotent-mcp"

	cr := makeTestMCPServer(crName, ns, "ghcr.io/test/mcp:1.0", 9090, 1)
	client, informer, stop := newTestSetup(t, ns, cr)
	defer stop()

	ctrl := NewMCPServerController(client, informer, ns, discardLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key := ns + "/" + crName

	// First reconcile.
	if err := ctrl.reconcile(ctx, key); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	// Second reconcile — should be idempotent.
	if err := ctrl.reconcile(ctx, key); err != nil {
		t.Errorf("second reconcile error: %v", err)
	}

	// Deployment still exists with correct name.
	deploy, err := client.Kubernetes.AppsV1().Deployments(ns).Get(ctx, "mcp-"+crName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment after second reconcile: %v", err)
	}
	if deploy.Name != "mcp-"+crName {
		t.Errorf("unexpected deployment name %q", deploy.Name)
	}
}

func TestReconcileCleanup(t *testing.T) {
	const ns = "test-ns"
	const crName = "cleanup-mcp"

	cr := makeTestMCPServer(crName, ns, "ghcr.io/test/mcp:1.0", 7070, 1)
	client, informer, stop := newTestSetup(t, ns, cr)
	defer stop()

	ctrl := NewMCPServerController(client, informer, ns, discardLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key := ns + "/" + crName

	// Create resources via reconcile.
	if err := ctrl.reconcile(ctx, key); err != nil {
		t.Fatalf("reconcile create: %v", err)
	}

	// Verify resources exist.
	if _, err := client.Kubernetes.AppsV1().Deployments(ns).Get(ctx, "mcp-"+crName, metav1.GetOptions{}); err != nil {
		t.Fatalf("deployment should exist: %v", err)
	}

	// Remove from indexer to simulate CR deletion.
	if err := informer.GetIndexer().Delete(cr); err != nil {
		t.Fatalf("delete from indexer: %v", err)
	}

	// Reconcile with the same key — object missing from store → cleanup.
	if err := ctrl.reconcile(ctx, key); err != nil {
		t.Fatalf("reconcile cleanup: %v", err)
	}

	// All three resources should now be gone.
	_, errD := client.Kubernetes.AppsV1().Deployments(ns).Get(ctx, "mcp-"+crName, metav1.GetOptions{})
	_, errS := client.Kubernetes.CoreV1().Services(ns).Get(ctx, "mcp-"+crName, metav1.GetOptions{})
	_, errN := client.Kubernetes.NetworkingV1().NetworkPolicies(ns).Get(ctx, "mcp-"+crName, metav1.GetOptions{})

	if !apierrors.IsNotFound(errD) {
		t.Errorf("deployment should be deleted, err=%v", errD)
	}
	if !apierrors.IsNotFound(errS) {
		t.Errorf("service should be deleted, err=%v", errS)
	}
	if !apierrors.IsNotFound(errN) {
		t.Errorf("networkpolicy should be deleted, err=%v", errN)
	}
}
