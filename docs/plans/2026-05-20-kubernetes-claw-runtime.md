# Kubernetes Claw Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the embedded Kubernetes sandbox controller MVP and replace the demo runtime loop with a tested Claw adapter execution boundary.

**Architecture:** Keep the backend-facing `sandbox.RuntimeProvider` abstraction and add a Kubernetes implementation behind it. The Kubernetes provider creates a token secret/reference, workspace PVC, generated NetworkPolicy, and runtime Pod; readiness remains runtime WebSocket registration. The runtime process receives `task.run`, normalizes it into an internal task, executes it through `ClawAdapter`, and maps adapter events back to gateway envelopes.

**Tech Stack:** Go 1.25, `github.com/gorilla/websocket`, Kubernetes `client-go`/API types, Helm chart values, Go unit tests with fakes.

---

## File Structure

- Create `internal/sandbox/secrets.go`: `RuntimeSecretStore`, `SecretRef`, Kubernetes Secret fallback implementation, and a no-op/file ref helper for tests.
- Create `internal/sandbox/networkpolicy.go`: sandbox-owned `NetworkPolicySpec`, `NetworkPolicyMode`, `EgressRule`, and `BuildRuntimeNetworkPolicy`.
- Create `internal/sandbox/k8s_resources.go`: core Kubernetes Pod/PVC/Secret object builders and label helpers.
- Create `internal/sandbox/kubernetes_provider.go`: embedded controller MVP implementing `RuntimeProvider` using Kubernetes clients.
- Create tests: `internal/sandbox/secrets_test.go`, `internal/sandbox/networkpolicy_test.go`, `internal/sandbox/k8s_resources_test.go`, `internal/sandbox/kubernetes_provider_test.go`.
- Modify `internal/sandbox/podspec.go` and `provider.go`: add sandbox/runtime labels, token file/env support, gateway URL, and runtime flavor envs.
- Modify `internal/config/config.go`, `cmd/shclop/main.go`, `internal/api/server.go`: expose Kubernetes provider and sandbox config.
- Create `internal/claw/adapter.go`, `internal/claw/subprocess.go`, `internal/claw/adapter_test.go`: runtime-side adapter contract and subprocess fallback.
- Modify `cmd/shclop-runtime/main.go`: replace hard-coded demo events with adapter-driven event streaming and token file support.
- Modify `charts/shclop/values.yaml` and `charts/shclop/templates/deployment.yaml`: add Kubernetes sandbox, NetworkPolicy, PVC, SecretStore, namespace, and gateway config values/env.
- Modify `README.md` and `docs/kubernetes-claw-runtime-design.md` only if implementation details diverge from the current spec.

## Task 1: Config Surface and Provider Wiring

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/shclop/main.go`
- Modify: `internal/api/server.go:92-100`
- Test: `internal/api/server_test.go`

- [ ] **Step 1: Write failing config/provider tests**

Add tests near existing server/provider config tests in `internal/api/server_test.go`:

```go
func TestSandboxProviderFromConfigSupportsKubernetes(t *testing.T) {
	cfg := config.Default()
	cfg.SandboxProvider = "kubernetes"
	cfg.KubernetesNamespace = "agents"
	cfg.KubernetesGatewayURL = "ws://shclop-backend:8080/runtime/ws"
	cfg.AgentRuntimeClassName = "kata-clh"
	cfg.RuntimeImages = map[string]string{
		"nanoclaw": "registry.example.com/shclop-runtime-nanoclaw:1",
		"openclaw": "registry.example.com/shclop-runtime-openclaw:1",
	}
	cfg.NetworkPolicyEnabled = true
	cfg.NetworkPolicyMode = "restricted"

	provider, err := sandboxProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("sandboxProviderFromConfig: %v", err)
	}
	if _, ok := provider.(*sandbox.KubernetesRuntimeProvider); !ok {
		t.Fatalf("expected KubernetesRuntimeProvider, got %T", provider)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./internal/api -run TestSandboxProviderFromConfigSupportsKubernetes -count=1`

Expected: FAIL because config fields and `KubernetesRuntimeProvider` do not exist.

- [ ] **Step 3: Add config fields and flags**

In `internal/config/config.go`, extend `Config` and `Default()`:

```go
AgentRuntimeClassName string
RuntimeImages         map[string]string
KubernetesNamespace   string
KubernetesGatewayURL  string
WorkspaceStorageClass string
WorkspaceSize         string
WorkspaceRetention    string
NetworkPolicyEnabled  bool
NetworkPolicyMode     string
NetworkPolicyCIDRs    string
SecretStore           string
```

Set defaults:

```go
AgentRuntimeClassName: env("SHCLOP_AGENT_RUNTIME_CLASS", "kata"),
RuntimeImages: map[string]string{
	"nanoclaw": env("SHCLOP_RUNTIME_IMAGE_NANOCLAW", "shclop-runtime-nanoclaw:latest"),
	"openclaw": env("SHCLOP_RUNTIME_IMAGE_OPENCLAW", "shclop-runtime-openclaw:latest"),
},
KubernetesNamespace:   env("SHCLOP_KUBERNETES_NAMESPACE", "default"),
KubernetesGatewayURL:  env("SHCLOP_KUBERNETES_GATEWAY_URL", "ws://shclop-backend:8080/runtime/ws"),
WorkspaceStorageClass: os.Getenv("SHCLOP_WORKSPACE_STORAGE_CLASS"),
WorkspaceSize:         env("SHCLOP_WORKSPACE_SIZE", "10Gi"),
WorkspaceRetention:    env("SHCLOP_WORKSPACE_RETENTION", "delete"),
NetworkPolicyEnabled:  envBool("SHCLOP_NETWORK_POLICY_ENABLED", true),
NetworkPolicyMode:     env("SHCLOP_NETWORK_POLICY_MODE", "restricted"),
NetworkPolicyCIDRs:    os.Getenv("SHCLOP_NETWORK_POLICY_ALLOWED_CIDRS"),
SecretStore:           env("SHCLOP_RUNTIME_SECRET_STORE", "kubernetes"),
```

Add helper:

```go
func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
```

Import `strings` in `internal/config/config.go`.

In `cmd/shclop/main.go`, add flags:

```go
flag.StringVar(&cfg.KubernetesNamespace, "kubernetes-namespace", cfg.KubernetesNamespace, "namespace for Kubernetes runtime sandboxes")
flag.StringVar(&cfg.KubernetesGatewayURL, "kubernetes-gateway-url", cfg.KubernetesGatewayURL, "gateway websocket URL passed to Kubernetes runtime pods")
flag.StringVar(&cfg.AgentRuntimeClassName, "agent-runtime-class", cfg.AgentRuntimeClassName, "Kubernetes RuntimeClass for agent runtime pods")
flag.StringVar(&cfg.WorkspaceStorageClass, "workspace-storage-class", cfg.WorkspaceStorageClass, "storage class for runtime workspace PVCs")
flag.StringVar(&cfg.WorkspaceSize, "workspace-size", cfg.WorkspaceSize, "runtime workspace PVC size")
flag.StringVar(&cfg.WorkspaceRetention, "workspace-retention", cfg.WorkspaceRetention, "workspace PVC cleanup policy: delete or retain")
flag.BoolVar(&cfg.NetworkPolicyEnabled, "network-policy", cfg.NetworkPolicyEnabled, "create NetworkPolicy for Kubernetes runtime pods")
flag.StringVar(&cfg.NetworkPolicyMode, "network-policy-mode", cfg.NetworkPolicyMode, "NetworkPolicy mode: disabled, restricted, custom")
flag.StringVar(&cfg.NetworkPolicyCIDRs, "network-policy-allowed-cidrs", cfg.NetworkPolicyCIDRs, "comma-separated CIDRs allowed for custom runtime egress")
flag.StringVar(&cfg.SecretStore, "runtime-secret-store", cfg.SecretStore, "runtime token secret store: kubernetes")
```

- [ ] **Step 4: Wire provider branch with a compile-safe provider shell**

In `internal/api/server.go`, add `case "kubernetes"`:

```go
case "kubernetes":
	return sandbox.NewKubernetesRuntimeProvider(sandbox.KubernetesRuntimeProviderConfig{
		Namespace:          cfg.KubernetesNamespace,
		GatewayURL:         cfg.KubernetesGatewayURL,
		RuntimeClassName:  cfg.AgentRuntimeClassName,
		Images:            cfg.RuntimeImages,
		WorkspaceSize:     cfg.WorkspaceSize,
		StorageClassName:  cfg.WorkspaceStorageClass,
		WorkspacePolicy:   cfg.WorkspaceRetention,
		SecretStore:       cfg.SecretStore,
		NetworkPolicySpec: sandbox.NetworkPolicySpecFromConfig(cfg.NetworkPolicyEnabled, cfg.NetworkPolicyMode, cfg.NetworkPolicyCIDRs),
	})
```

If import cycles appear because `sandbox` would import `config`, keep `NetworkPolicySpecFromConfig` in package `sandbox` with primitive arguments as shown.

Create the provider shell in `internal/sandbox/kubernetes_provider.go`; Task 4 will replace the `Start`/`Stop` internals with Kubernetes API calls:

```go
package sandbox

import (
	"context"
	"errors"
)

type KubernetesRuntimeProviderConfig struct {
	Namespace         string
	GatewayURL        string
	RuntimeClassName string
	Images            map[string]string
	WorkspaceSize     string
	StorageClassName  string
	WorkspacePolicy   string
	SecretStore       string
	NetworkPolicySpec NetworkPolicySpec
}

type KubernetesRuntimeProvider struct {
	cfg KubernetesRuntimeProviderConfig
}

func NewKubernetesRuntimeProvider(cfg KubernetesRuntimeProviderConfig) (*KubernetesRuntimeProvider, error) {
	return &KubernetesRuntimeProvider{cfg: cfg}, nil
}

func (p *KubernetesRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
	return RuntimeLease{}, errors.New("kubernetes runtime provider start is not wired")
}

func (p *KubernetesRuntimeProvider) Stop(ctx context.Context, agentID string) error {
	return errors.New("kubernetes runtime provider stop is not wired")
}
```

- [ ] **Step 5: Run package tests**

Run: `go test ./internal/api -count=1`

Expected: PASS. The Kubernetes provider shell compiles and is selected, but it returns explicit runtime errors until Task 4 wires Kubernetes API calls.

## Task 2: SecretRef and Runtime Token Delivery

**Files:**
- Create: `internal/sandbox/secrets.go`
- Create: `internal/sandbox/secrets_test.go`
- Modify: `internal/sandbox/podspec.go`
- Modify: `cmd/shclop-runtime/main.go`

- [ ] **Step 1: Write failing SecretStore tests**

Create `internal/sandbox/secrets_test.go`:

```go
package sandbox

import "testing"

func TestKubernetesSecretStoreBuildsSecretRef(t *testing.T) {
	store := KubernetesSecretStore{Namespace: "agents"}
	ref, secret, err := store.BuildRuntimeTokenSecret("agent-1", "token-1")
	if err != nil {
		t.Fatalf("BuildRuntimeTokenSecret: %v", err)
	}
	if ref.Name != "shclop-runtime-token-agent-1" || ref.Key != "token" || ref.MountPath != "/var/run/shclop/token" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	if string(secret.StringData["token"]) != "token-1" {
		t.Fatalf("secret token not set: %#v", secret.StringData)
	}
}
```

- [ ] **Step 2: Add SecretRef and builder**

Create `internal/sandbox/secrets.go`:

```go
package sandbox

import (
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretRef struct {
	Name      string
	Key       string
	EnvName   string
	MountPath string
}

type RuntimeSecretStore interface {
	BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error)
	DeleteRuntimeTokenName(agentID string) string
}

type KubernetesSecretStore struct {
	Namespace string
}

func (s KubernetesSecretStore) BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || strings.TrimSpace(token) == "" {
		return SecretRef{}, nil, errors.New("agent id and runtime token are required")
	}
	name := "shclop-runtime-token-" + agentID
	ref := SecretRef{Name: name, Key: "token", EnvName: "SHCLOP_RUNTIME_TOKEN", MountPath: "/var/run/shclop/token"}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace, Labels: RuntimeLabels(agentID, "", "")},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"token": token},
	}
	return ref, secret, nil
}

func (s KubernetesSecretStore) DeleteRuntimeTokenName(agentID string) string {
	return "shclop-runtime-token-" + strings.TrimSpace(agentID)
}
```

- [ ] **Step 3: Update pod request for token file**

In `internal/sandbox/podspec.go`, add to `AgentPodRequest`:

```go
GatewayURL string
Runtime    string
SandboxID  string
SecretRef  SecretRef
```

Add to `ContainerSpec`:

```go
Env map[string]string
```

In `BuildAgentPodSpec`, populate env:

```go
Env: map[string]string{
	"SHCLOP_GATEWAY_URL":        req.GatewayURL,
	"SHCLOP_AGENT_ID":           req.AgentID,
	"SHCLOP_AGENT_FLAVOR":       req.Runtime,
	"SHCLOP_RUNTIME_TOKEN_FILE": req.SecretRef.MountPath,
},
```

Keep `SHCLOP_RUNTIME_TOKEN` out of the Pod spec when a token file is configured.

- [ ] **Step 4: Add token-file support to runtime main**

In `cmd/shclop-runtime/main.go`, replace token initialization with:

```go
token := flag.String("token", runtimeTokenFromEnv(), "runtime token returned by agent start")
```

Add helper:

```go
func runtimeTokenFromEnv() string {
	if path := strings.TrimSpace(os.Getenv("SHCLOP_RUNTIME_TOKEN_FILE")); path != "" {
		content, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return os.Getenv("SHCLOP_RUNTIME_TOKEN")
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/sandbox ./cmd/shclop-runtime -count=1`

Expected: PASS.

## Task 3: Kubernetes Resource Builders

**Files:**
- Create: `internal/sandbox/networkpolicy.go`
- Create: `internal/sandbox/networkpolicy_test.go`
- Create: `internal/sandbox/k8s_resources.go`
- Create: `internal/sandbox/k8s_resources_test.go`
- Modify: `internal/sandbox/podspec_test.go`

- [ ] **Step 1: Write NetworkPolicy tests**

Create `internal/sandbox/networkpolicy_test.go`:

```go
package sandbox

import "testing"

func TestBuildRuntimeNetworkPolicyRestricted(t *testing.T) {
	spec := NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted, AllowBackend: true, AllowVault: true}
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", spec)
	if err != nil {
		t.Fatalf("BuildRuntimeNetworkPolicy: %v", err)
	}
	if policy.Name != "shclop-runtime-netpol-agent-1" {
		t.Fatalf("unexpected name %q", policy.Name)
	}
	if policy.Labels["shclop.io/agent-id"] != "agent-1" || policy.Labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("missing labels: %#v", policy.Labels)
	}
	if len(policy.Spec.Egress) == 0 {
		t.Fatal("restricted policy should include explicit egress rules")
	}
}

func TestBuildRuntimeNetworkPolicyDisabled(t *testing.T) {
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", NetworkPolicySpec{Enabled: false, Mode: NetworkPolicyDisabled})
	if err != nil {
		t.Fatalf("BuildRuntimeNetworkPolicy disabled: %v", err)
	}
	if policy != nil {
		t.Fatalf("disabled mode should not create policy: %#v", policy)
	}
}
```

- [ ] **Step 2: Implement NetworkPolicy builder**

Create `internal/sandbox/networkpolicy.go` with:

```go
package sandbox

import (
	"net"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkPolicyMode string

const (
	NetworkPolicyDisabled   NetworkPolicyMode = "disabled"
	NetworkPolicyRestricted NetworkPolicyMode = "restricted"
	NetworkPolicyCustom     NetworkPolicyMode = "custom"
)

type EgressRule struct {
	Name    string
	CIDR    string
	DNSName string
	Ports   []int32
}

type NetworkPolicySpec struct {
	Enabled       bool
	Mode          NetworkPolicyMode
	AllowBackend  bool
	AllowVault    bool
	AllowedEgress []EgressRule
}

func NetworkPolicySpecFromConfig(enabled bool, mode string, cidrs string) NetworkPolicySpec {
	spec := NetworkPolicySpec{Enabled: enabled, Mode: NetworkPolicyMode(strings.TrimSpace(mode)), AllowBackend: true, AllowVault: true}
	if spec.Mode == "" {
		spec.Mode = NetworkPolicyRestricted
	}
	for _, raw := range strings.Split(cidrs, ",") {
		cidr := strings.TrimSpace(raw)
		if cidr != "" {
			spec.AllowedEgress = append(spec.AllowedEgress, EgressRule{Name: "custom-" + strconv.Itoa(len(spec.AllowedEgress)+1), CIDR: cidr, Ports: []int32{443}})
		}
	}
	return spec
}

func BuildRuntimeNetworkPolicy(agentID, sandboxID string, spec NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {
	if !spec.Enabled || spec.Mode == NetworkPolicyDisabled {
		return nil, nil
	}
	labels := RuntimeLabels(agentID, "", sandboxID)
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "shclop-runtime-netpol-" + agentID, Labels: labels},
		Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"shclop.io/agent-id": agentID}}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}},
	}
	for _, allowed := range spec.AllowedEgress {
		if allowed.CIDR == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(allowed.CIDR)
		if err != nil {
			return nil, err
		}
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: cidr.String()}}}})
	}
	if len(policy.Spec.Egress) == 0 {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{})
	}
	return policy, nil
}
```

- [ ] **Step 3: Write PVC/Pod builder tests**

Create `internal/sandbox/k8s_resources_test.go`:

```go
package sandbox

import "testing"

func TestBuildWorkspacePVC(t *testing.T) {
	pvc := BuildWorkspacePVC("agent-1", "sandbox-1", "fast", "5Gi")
	if pvc.Name != "shclop-workspace-agent-1" {
		t.Fatalf("unexpected pvc name %q", pvc.Name)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast" {
		t.Fatalf("storage class not set: %#v", pvc.Spec.StorageClassName)
	}
}

func TestRuntimeLabelsIncludeExtractionFields(t *testing.T) {
	labels := RuntimeLabels("agent-1", "nanoclaw", "sandbox-1")
	for _, key := range []string{"app.kubernetes.io/name", "shclop.io/agent-id", "shclop.io/runtime-flavor", "shclop.io/sandbox-id"} {
		if labels[key] == "" {
			t.Fatalf("missing label %s in %#v", key, labels)
		}
	}
}
```

- [ ] **Step 4: Implement resource builders**

Create `internal/sandbox/k8s_resources.go` with `RuntimeLabels`, `BuildWorkspacePVC`, and `BuildRuntimePod` converting `AgentPodSpec` into `corev1.Pod`. Use `resource.MustParse(size)` for storage, `corev1.PersistentVolumeFilesystem`, `corev1.PullIfNotPresent`, `corev1.SeccompProfileTypeRuntimeDefault`, and `corev1.Capability("ALL")`.

Update `BuildAgentPodSpec` labels to call `RuntimeLabels(req.AgentID, req.Runtime, req.SandboxID)` and preserve `shclop.io/owner-id`.

- [ ] **Step 5: Run resource tests**

Run: `go test ./internal/sandbox -run 'Test(BuildRuntimeNetworkPolicy|BuildWorkspacePVC|RuntimeLabels|AgentPodSpec)' -count=1`

Expected: PASS.

## Task 4: Embedded Kubernetes RuntimeProvider

**Files:**
- Create: `internal/sandbox/kubernetes_provider.go`
- Create: `internal/sandbox/kubernetes_provider_test.go`
- Modify: `go.mod`, `go.sum`
- Modify: `internal/sandbox/provider.go`

- [ ] **Step 1: Write provider fake-client tests**

Create `internal/sandbox/kubernetes_provider_test.go`:

```go
package sandbox

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesRuntimeProviderStartCreatesSandboxResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := &KubernetesRuntimeProvider{
		client: client,
		cfg: KubernetesRuntimeProviderConfig{
			Namespace: "agents", GatewayURL: "ws://backend/runtime/ws", RuntimeClassName: "kata",
			Images: map[string]string{"nanoclaw": "runtime:nano"}, WorkspaceSize: "1Gi", WorkspacePolicy: "delete",
			NetworkPolicySpec: NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted, AllowBackend: true},
		},
		secretStore: KubernetesSecretStore{Namespace: "agents"},
	}
	lease, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", OwnerID: "user-1", Runtime: "nanoclaw", RuntimeToken: "token-1"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if lease.Provider != "kubernetes" || lease.ExternalID == "" {
		t.Fatalf("unexpected lease: %#v", lease)
	}
	if _, err := client.CoreV1().Secrets("agents").Get(context.Background(), "shclop-runtime-token-agent-1", metav1.GetOptions{}); err != nil {
		t.Fatalf("secret not created: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("agents").Get(context.Background(), "shclop-workspace-agent-1", metav1.GetOptions{}); err != nil {
		t.Fatalf("pvc not created: %v", err)
	}
	if _, err := client.CoreV1().Pods("agents").Get(context.Background(), "agent-agent-1", metav1.GetOptions{}); err != nil {
		t.Fatalf("pod not created: %v", err)
	}
}
```

Include `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"` in imports.

- [ ] **Step 2: Add Kubernetes dependencies**

Run:

```bash
go get k8s.io/api@latest k8s.io/apimachinery@latest k8s.io/client-go@latest
```

Expected: `go.mod` and `go.sum` update.

- [ ] **Step 3: Implement provider config and constructor**

Create `internal/sandbox/kubernetes_provider.go` with:

```go
type KubernetesRuntimeProviderConfig struct {
	Namespace          string
	GatewayURL         string
	RuntimeClassName  string
	Images             map[string]string
	WorkspaceSize      string
	StorageClassName   string
	WorkspacePolicy    string
	SecretStore        string
	NetworkPolicySpec  NetworkPolicySpec
}

type KubernetesRuntimeProvider struct {
	cfg         KubernetesRuntimeProviderConfig
	client      kubernetes.Interface
	secretStore RuntimeSecretStore
}
```

Implement `NewKubernetesRuntimeProvider(cfg)` using in-cluster config first and falling back to `$KUBECONFIG` via `clientcmd.BuildConfigFromFlags`. Return clear errors for missing config.

- [ ] **Step 4: Implement Start**

`Start` must:
1. normalize and validate runtime using existing `normalizeRuntime` / `isKnownRuntime`;
2. resolve image from `cfg.Images`;
3. generate `sandboxID := "sandbox-" + request.AgentID`;
4. build Secret, PVC, NetworkPolicy, and Pod;
5. create/update resources idempotently with create then update-on-already-exists;
6. return `RuntimeLease{AgentID: request.AgentID, Provider: "kubernetes", Runtime: runtime, ExternalID: pod.Name}`.

- [ ] **Step 5: Implement Stop**

`Stop` must delete Pod, NetworkPolicy, runtime Secret, and PVC only when `WorkspacePolicy == "delete"`. Treat Kubernetes NotFound errors as success.

- [ ] **Step 6: Run provider tests**

Run: `go test ./internal/sandbox -run TestKubernetesRuntimeProvider -count=1`

Expected: PASS.

## Task 5: Claw Adapter Contract and Subprocess Fallback

**Files:**
- Create: `internal/claw/adapter.go`
- Create: `internal/claw/subprocess.go`
- Create: `internal/claw/adapter_test.go`

- [ ] **Step 1: Write adapter tests**

Create `internal/claw/adapter_test.go`:

```go
package claw

import (
	"context"
	"testing"
)

func TestDemoAdapterEmitsStructuredEvents(t *testing.T) {
	adapter := DemoAdapter{Flavor: "nanoclaw"}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var types []EventType
	for event := range events {
		types = append(types, event.Type)
	}
	want := []EventType{EventStarted, EventDelta, EventDelta, EventDone}
	if len(types) != len(want) {
		t.Fatalf("got %v want %v", types, want)
	}
}
```

- [ ] **Step 2: Implement adapter types**

Create `internal/claw/adapter.go`:

```go
package claw

import "context"

type Task struct { Text string }
type EventType string

const (
	EventStarted EventType = "started"
	EventDelta   EventType = "delta"
	EventDone    EventType = "done"
	EventError   EventType = "error"
)

type Event struct { Type EventType; Text string; Err error; ExitCode int }

type Adapter interface { Run(ctx context.Context, task Task) (<-chan Event, error) }
```

Add `DemoAdapter` preserving current behavior so demo tests remain stable.

- [ ] **Step 3: Implement subprocess fallback**

Create `internal/claw/subprocess.go` with `SubprocessAdapter{Binary string; Args []string; Env []string}`. It should start the process with task text on stdin, stream stdout lines as `EventDelta`, stderr lines as `EventDelta` prefixed with `stderr: `, emit `EventDone` on exit 0, and `EventError` on non-zero exit.

- [ ] **Step 4: Run adapter tests**

Run: `go test ./internal/claw -count=1`

Expected: PASS.

## Task 6: Runtime Main Uses ClawAdapter

**Files:**
- Modify: `cmd/shclop-runtime/main.go`
- Create: `cmd/shclop-runtime/main_test.go`

- [ ] **Step 1: Write runtime helper tests**

Create `cmd/shclop-runtime/main_test.go`:

```go
package main

import "testing"

func TestTaskTextExtractsPayloadText(t *testing.T) {
	text := taskText(map[string]any{"text": "hello"})
	if text != "hello" {
		t.Fatalf("got %q", text)
	}
}
```

- [ ] **Step 2: Refactor runtime loop**

In `cmd/shclop-runtime/main.go`, add:

```go
func adapterForRuntime(runtimeName string) claw.Adapter {
	return claw.DemoAdapter{Flavor: runtimeName}
}

func taskText(payload map[string]any) string {
	text, _ := payload["text"].(string)
	return text
}
```

Replace the hard-coded `events := []gateway.Envelope{...}` block with adapter execution:

```go
events, err := adapterForRuntime(*runtimeName).Run(context.Background(), claw.Task{Text: taskText(task.Payload)})
if err != nil { /* write message.error */ }
seq := 1
for event := range events {
	envelope := clawEventToEnvelope(event, task, seq)
	seq++
	if err := conn.WriteJSON(envelope); err != nil { log.Fatal(err) }
}
```

Add `clawEventToEnvelope` mapping `EventStarted` → `message.started`, `EventDelta` → `message.delta`, `EventDone` → `message.done`, `EventError` → `message.error`.

- [ ] **Step 3: Run runtime and API demo tests**

Run: `go test ./cmd/shclop-runtime ./internal/api -run 'TestTaskTextExtractsPayloadText|TestFunctionalDemoRoutesBrowserTaskThroughRuntime' -count=1`

Expected: PASS.

## Task 7: Helm Values and Documentation Sync

**Files:**
- Modify: `charts/shclop/values.yaml`
- Modify: `charts/shclop/templates/deployment.yaml`
- Modify: `README.md`
- Modify: `docs/kubernetes-claw-runtime-design.md` only if implementation differs

- [ ] **Step 1: Add Helm values**

Extend `charts/shclop/values.yaml`:

```yaml
sandbox:
  provider: mock
  dockerGatewayURL: ws://host.docker.internal:8080/runtime/ws
  runtimeImagePrefix: shclop-runtime
  kubernetes:
    namespace: default
    gatewayURL: ws://shclop-backend:8080/runtime/ws
    workspace:
      size: 10Gi
      storageClassName: ""
      retention: delete
    secretStore: kubernetes
    networkPolicy:
      enabled: true
      mode: restricted
      allowedCIDRs: ""
```

- [ ] **Step 2: Pass CLI args/env in deployment template**

Add args to `charts/shclop/templates/deployment.yaml`:

```yaml
- "--kubernetes-namespace={{ .Values.sandbox.kubernetes.namespace }}"
- "--kubernetes-gateway-url={{ .Values.sandbox.kubernetes.gatewayURL }}"
- "--workspace-size={{ .Values.sandbox.kubernetes.workspace.size }}"
- "--workspace-storage-class={{ .Values.sandbox.kubernetes.workspace.storageClassName }}"
- "--workspace-retention={{ .Values.sandbox.kubernetes.workspace.retention }}"
- "--runtime-secret-store={{ .Values.sandbox.kubernetes.secretStore }}"
- "--network-policy={{ .Values.sandbox.kubernetes.networkPolicy.enabled }}"
- "--network-policy-mode={{ .Values.sandbox.kubernetes.networkPolicy.mode }}"
- "--network-policy-allowed-cidrs={{ .Values.sandbox.kubernetes.networkPolicy.allowedCIDRs }}"
- "--agent-runtime-class={{ .Values.agentRuntime.runtimeClassName }}"
```

- [ ] **Step 3: Update docs**

README should state that `sandbox.provider=kubernetes` now creates Pod + Secret + PVC + NetworkPolicy resources, but production Vault/egress proxy and standalone controller extraction remain future work.

- [ ] **Step 4: Run docs/template checks**

Run: `git diff --check -- README.md docs/kubernetes-claw-runtime-design.md charts/shclop/values.yaml charts/shclop/templates/deployment.yaml`

Expected: no output.

## Task 8: Full Verification

**Files:**
- All changed files.

- [ ] **Step 1: Run all Go tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run whitespace check**

Run: `git diff --check`

Expected: no output.

- [ ] **Step 3: Inspect final diff**

Run: `git diff --stat && git diff -- README.md docs/kubernetes-claw-runtime-design.md internal config cmd charts go.mod go.sum`

Expected: diff only includes planned implementation, docs, Helm values, and dependency updates.

## Self-Review

- Spec coverage: embedded Kubernetes provider, SecretRef, NetworkPolicy interface, PVC/workspace, cleanup, runtime token file, and ClawAdapter are mapped to Tasks 1-7.
- Deferred by design: standalone controller, Vault implementation, full egress proxy, dynamic per-agent egress approval, real upstream Claw structured protocol beyond subprocess fallback.
- Placeholder scan: no task uses unresolved placeholders as acceptance criteria; deferred work is explicitly out of MVP scope.
- Type consistency: `RuntimeProvider`, `StartRequest`, `RuntimeLease`, `SecretRef`, `NetworkPolicySpec`, `ClawAdapter`, `Task`, and `Event` names are consistent across tasks.
