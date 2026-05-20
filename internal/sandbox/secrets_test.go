package sandbox

import "testing"

func TestKubernetesSecretStoreBuildsSecretRef(t *testing.T) {
	store := KubernetesSecretStore{Namespace: "default"}
	ref, secret, err := store.BuildRuntimeTokenSecret("agent-1", "  tok-123  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Name != "shclop-runtime-token-agent-1" || ref.Key != "token" || ref.MountPath != "/var/run/shclop/token" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
	if secret == nil || secret.StringData["token"] != "tok-123" {
		t.Fatalf("unexpected secret data: %#v", secret)
	}
}

func TestKubernetesSecretStoreValidatesInputs(t *testing.T) {
	store := KubernetesSecretStore{}
	if _, _, err := store.BuildRuntimeTokenSecret("", "tok"); err == nil {
		t.Fatal("expected error for empty agentID")
	}
	if _, _, err := store.BuildRuntimeTokenSecret("agent-1", " "); err == nil {
		t.Fatal("expected error for empty token")
	}
}
