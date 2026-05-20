package sandbox

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestKubernetesRuntimeProviderStartCreatesSandboxResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := &KubernetesRuntimeProvider{
		cfg: KubernetesRuntimeProviderConfig{
			Namespace:        "default",
			GatewayURL:       "ws://gateway",
			RuntimeClassName: "kata",
			Images: map[string]string{
				"openclaw": "image-openclaw:latest",
			},
			WorkspaceSize:     "20Gi",
			StorageClassName:  "fast",
			WorkspacePolicy:   "delete",
			SecretStore:       "kubernetes",
			NetworkPolicySpec: NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted},
		},
		client:      client,
		secretStore: KubernetesSecretStore{Namespace: "default"},
	}

	lease, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", OwnerID: "user-1", Runtime: "openclaw", RuntimeToken: "tok-123"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if lease.Provider != "kubernetes" || lease.ExternalID != "agent-agent-1" {
		t.Fatalf("unexpected lease: %#v", lease)
	}

	secret, err := client.CoreV1().Secrets("default").Get(context.Background(), "shclop-runtime-token-agent-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if secret.StringData["token"] != "tok-123" && string(secret.Data["token"]) != "tok-123" {
		t.Fatalf("unexpected secret token data: %#v %#v", secret.StringData, secret.Data)
	}

	pvc, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), "shclop-workspace-agent-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pvc: %v", err)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast" {
		t.Fatalf("unexpected pvc: %#v", pvc.Spec.StorageClassName)
	}

	np, err := client.NetworkingV1().NetworkPolicies("default").Get(context.Background(), "shclop-runtime-netpol-agent-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get networkpolicy: %v", err)
	}
	if np.Namespace != "default" || np.Spec.PodSelector.MatchLabels["shclop.io/agent-id"] != "agent-1" {
		t.Fatalf("unexpected networkpolicy: %#v", np)
	}

	pod, err := client.CoreV1().Pods("default").Get(context.Background(), "agent-agent-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata" {
		t.Fatalf("unexpected runtime class: %#v", pod.Spec.RuntimeClassName)
	}
	if got := pod.Labels["shclop.io/owner-id"]; got == "" || strings.ContainsAny(got, "|@") {
		t.Fatalf("expected sanitized owner label, got %#v", pod.Labels)
	}
	if pod.Spec.Containers[0].Env[0].Name != "SHCLOP_AGENT_FLAVOR" && pod.Spec.Containers[0].Env[0].Name != "SHCLOP_AGENT_ID" {
		t.Fatalf("unexpected envs: %#v", pod.Spec.Containers[0].Env)
	}
	if len(pod.Spec.Volumes) < 3 || pod.Spec.Volumes[2].Secret == nil {
		t.Fatalf("expected secret volume, got %#v", pod.Spec.Volumes)
	}

	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", OwnerID: "user-1", Runtime: "openclaw", RuntimeToken: "tok-123"}); err != nil {
		t.Fatalf("second start: %v", err)
	}
}

func TestKubernetesRuntimeProviderStartSanitizesLabelsForUnsafeAgentID(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := &KubernetesRuntimeProvider{
		cfg: KubernetesRuntimeProviderConfig{
			Namespace:         "default",
			GatewayURL:        "ws://gateway",
			RuntimeClassName:  "kata",
			Images:            map[string]string{"openclaw": "image-openclaw:latest"},
			WorkspacePolicy:   "delete",
			SecretStore:       "kubernetes",
			NetworkPolicySpec: NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted},
		},
		client:      client,
		secretStore: KubernetesSecretStore{Namespace: "default"},
	}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent|bad@example.com", OwnerID: "oidc|alice@example.com", Runtime: "openclaw", RuntimeToken: "tok-123"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	secret, _ := client.CoreV1().Secrets("default").Get(context.Background(), "shclop-runtime-token-agent|bad@example.com", metav1.GetOptions{})
	if got := secret.Labels["shclop.io/agent-id"]; got == "" || strings.ContainsAny(got, "|@") {
		t.Fatalf("expected sanitized secret label, got %#v", secret.Labels)
	}
	np, _ := client.NetworkingV1().NetworkPolicies("default").Get(context.Background(), "shclop-runtime-netpol-agent|bad@example.com", metav1.GetOptions{})
	if got := np.Spec.PodSelector.MatchLabels["shclop.io/agent-id"]; got == "" || strings.ContainsAny(got, "|@") {
		t.Fatalf("expected sanitized selector, got %#v", np.Spec.PodSelector.MatchLabels)
	}
	pod, _ := client.CoreV1().Pods("default").Get(context.Background(), "agent-agent|bad@example.com", metav1.GetOptions{})
	if pod.Labels["shclop.io/agent-id"] != secret.Labels["shclop.io/agent-id"] || pod.Labels["shclop.io/agent-id"] != np.Spec.PodSelector.MatchLabels["shclop.io/agent-id"] {
		t.Fatalf("expected matching sanitized labels, got pod=%#v secret=%#v np=%#v", pod.Labels, secret.Labels, np.Spec.PodSelector.MatchLabels)
	}
}

func TestKubernetesRuntimeProviderStartDoesNotReplaceExistingPodOrPVCSpec(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent-agent-1", Namespace: "default", Labels: map[string]string{"existing": "pod"}}, Spec: corev1.PodSpec{NodeName: "keep-me"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "shclop-workspace-agent-1", Namespace: "default", Labels: map[string]string{"existing": "pvc"}}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "keep-me"}},
	)
	provider := &KubernetesRuntimeProvider{cfg: KubernetesRuntimeProviderConfig{Namespace: "default", GatewayURL: "ws://gateway", RuntimeClassName: "kata", Images: map[string]string{"openclaw": "image-openclaw:latest"}, WorkspaceSize: "20Gi", StorageClassName: "fast", SecretStore: "kubernetes"}, client: client, secretStore: KubernetesSecretStore{Namespace: "default"}}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", OwnerID: "user-1", Runtime: "openclaw", RuntimeToken: "tok-123"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	pod, _ := client.CoreV1().Pods("default").Get(context.Background(), "agent-agent-1", metav1.GetOptions{})
	if pod.Spec.NodeName != "keep-me" {
		t.Fatalf("expected pod spec to remain unchanged, got %#v", pod.Spec)
	}
	pvc, _ := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), "shclop-workspace-agent-1", metav1.GetOptions{})
	if pvc.Spec.VolumeName != "keep-me" {
		t.Fatalf("expected pvc spec to remain unchanged, got %#v", pvc.Spec)
	}
}

func TestKubernetesRuntimeProviderStartValidatesInputs(t *testing.T) {
	provider := &KubernetesRuntimeProvider{cfg: KubernetesRuntimeProviderConfig{Images: map[string]string{"openclaw": "image"}}, client: fake.NewSimpleClientset(), secretStore: KubernetesSecretStore{Namespace: "default"}}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", Runtime: "openclaw"}); err == nil {
		t.Fatal("expected runtime token error")
	}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", Runtime: "missing", RuntimeToken: "tok"}); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
	provider.cfg.Images = map[string]string{}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", Runtime: "openclaw", RuntimeToken: "tok"}); err == nil {
		t.Fatal("expected missing image error")
	}
}

func TestKubernetesRuntimeProviderStopDeletesResources(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent-agent-1", Namespace: "default"}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "shclop-runtime-netpol-agent-1", Namespace: "default"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shclop-runtime-token-agent-1", Namespace: "default"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "shclop-workspace-agent-1", Namespace: "default"}},
	)
	provider := &KubernetesRuntimeProvider{cfg: KubernetesRuntimeProviderConfig{Namespace: "default", WorkspacePolicy: "delete"}, client: client, secretStore: KubernetesSecretStore{Namespace: "default"}}
	if err := provider.Stop(context.Background(), "agent-1"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	checks := []struct{ kind, name string }{{"pod", "agent-agent-1"}, {"networkpolicy", "shclop-runtime-netpol-agent-1"}, {"secret", "shclop-runtime-token-agent-1"}, {"pvc", "shclop-workspace-agent-1"}}
	for _, check := range checks {
		var err error
		switch check.kind {
		case "pod":
			_, err = client.CoreV1().Pods("default").Get(context.Background(), check.name, metav1.GetOptions{})
		case "networkpolicy":
			_, err = client.NetworkingV1().NetworkPolicies("default").Get(context.Background(), check.name, metav1.GetOptions{})
		case "secret":
			_, err = client.CoreV1().Secrets("default").Get(context.Background(), check.name, metav1.GetOptions{})
		case "pvc":
			_, err = client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), check.name, metav1.GetOptions{})
		}
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected %s to be deleted, got err=%v", check.kind, err)
		}
	}
}

func TestKubernetesRuntimeProviderStopRetainsPVC(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "shclop-workspace-agent-1", Namespace: "default"}})
	provider := &KubernetesRuntimeProvider{cfg: KubernetesRuntimeProviderConfig{Namespace: "default", WorkspacePolicy: "retain"}, client: client, secretStore: KubernetesSecretStore{Namespace: "default"}}
	if err := provider.Stop(context.Background(), "agent-1"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), "shclop-workspace-agent-1", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected pvc retained, got %v", err)
	}
}

func TestKubernetesRuntimeProviderStopReturnsDeleteErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.Fake.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(corev1.Resource("pods"), "agent-agent-1", nil)
	})
	provider := &KubernetesRuntimeProvider{cfg: KubernetesRuntimeProviderConfig{Namespace: "default", WorkspacePolicy: "delete"}, client: client, secretStore: KubernetesSecretStore{Namespace: "default"}}
	if err := provider.Stop(context.Background(), "agent-1"); err == nil {
		t.Fatal("expected delete error")
	}
}
