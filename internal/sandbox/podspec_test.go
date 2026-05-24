package sandbox

import (
	"strings"
	"testing"
)

func TestAgentPodSpecUsesKataAndHardening(t *testing.T) {
	spec := BuildAgentPodSpec(AgentPodRequest{
		AgentID:          "agent-1",
		OwnerID:          "user-1",
		Image:            "shclop-runtime-openclaw:latest",
		RuntimeClassName: "kata",
		Runtime:          "openclaw",
		SandboxID:        "sandbox-1",
		WorkspacePVC:     "agent-1-workspace",
		CPU:              "1000m",
		Memory:           "1Gi",
		SecretRef:        SecretRef{Name: "shclop-runtime-token-agent-1", MountPath: "/var/run/shclop/token"},
	})

	if spec.RuntimeClassName != "kata" {
		t.Fatalf("expected runtime class kata, got %q", spec.RuntimeClassName)
	}
	if spec.AutomountServiceAccountToken {
		t.Fatal("agent pods must not mount service account tokens")
	}
	if spec.Container.Privileged {
		t.Fatal("agent container must not be privileged")
	}
	if spec.Container.AllowPrivilegeEscalation {
		t.Fatal("agent container must not allow privilege escalation")
	}
	if !spec.Container.RunAsNonRoot || spec.Container.RunAsUser == 0 {
		t.Fatalf("expected non-root runtime user, got nonRoot=%v uid=%d", spec.Container.RunAsNonRoot, spec.Container.RunAsUser)
	}
	if spec.Container.SeccompProfile != "RuntimeDefault" {
		t.Fatalf("expected RuntimeDefault seccomp profile, got %q", spec.Container.SeccompProfile)
	}
	if spec.HostNetwork || spec.HostPID || spec.HostIPC {
		t.Fatalf("expected host namespaces disabled, got network=%v pid=%v ipc=%v", spec.HostNetwork, spec.HostPID, spec.HostIPC)
	}
	if spec.FSGroup != 10001 {
		t.Fatalf("expected fsGroup 10001 for writable PVCs, got %d", spec.FSGroup)
	}
	if spec.Container.Image != "shclop-runtime-openclaw:latest" {
		t.Fatalf("unexpected image %q", spec.Container.Image)
	}
	if spec.Container.VolumeMounts[0].MountPath != "/workspace" {
		t.Fatalf("expected workspace mount, got %#v", spec.Container.VolumeMounts)
	}
	if spec.Labels["shclop.io/agent-id"] != "agent-1" {
		t.Fatalf("missing agent label: %#v", spec.Labels)
	}
	if spec.Labels["shclop.io/runtime-flavor"] != "openclaw" || spec.Labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("missing runtime labels: %#v", spec.Labels)
	}
	if spec.Labels["shclop.io/owner-id"] != "user-1" {
		t.Fatalf("missing owner label: %#v", spec.Labels)
	}
	if spec.Container.Env["SHCLOP_RUNTIME_TOKEN_FILE"] != "/var/run/shclop/token" {
		t.Fatalf("expected token file env, got %#v", spec.Container.Env)
	}
	if _, ok := spec.Container.Env["SHCLOP_RUNTIME_TOKEN"]; ok {
		t.Fatal("did not expect runtime token env when token file is configured")
	}
	if spec.Container.Env["SHCLOP_AGENT_FLAVOR"] != "openclaw" {
		t.Fatalf("expected runtime flavor env, got %#v", spec.Container.Env)
	}
	if len(spec.Volumes) != 3 {
		t.Fatalf("expected runtime token volume added, got %#v", spec.Volumes)
	}
	if spec.Volumes[2].Name != "runtime-token" || spec.Volumes[2].SecretName != "shclop-runtime-token-agent-1" || spec.Volumes[2].SecretKey != "token" || spec.Volumes[2].SecretPath != "token" {
		t.Fatalf("unexpected runtime token volume: %#v", spec.Volumes[2])
	}
	if !spec.Container.VolumeMounts[2].ReadOnly || spec.Container.VolumeMounts[2].MountPath != "/var/run/shclop" {
		t.Fatalf("unexpected runtime token mount: %#v", spec.Container.VolumeMounts[2])
	}
}

func TestAgentPodSpecSecretMountDirUsesFilePath(t *testing.T) {
	spec := BuildAgentPodSpec(AgentPodRequest{AgentID: "agent-1", Image: "img", SecretRef: SecretRef{Name: "secret", Key: "custom-key", MountPath: "/var/run/shclop/token"}})
	if spec.Container.Env["SHCLOP_RUNTIME_TOKEN_FILE"] != "/var/run/shclop/token" {
		t.Fatalf("unexpected token file env: %#v", spec.Container.Env)
	}
	if spec.Container.VolumeMounts[len(spec.Container.VolumeMounts)-1].MountPath != "/var/run/shclop" {
		t.Fatalf("unexpected mount dir: %#v", spec.Container.VolumeMounts)
	}
	if spec.Volumes[len(spec.Volumes)-1].SecretKey != "custom-key" || spec.Volumes[len(spec.Volumes)-1].SecretPath != "token" {
		t.Fatalf("unexpected projected secret: %#v", spec.Volumes[len(spec.Volumes)-1])
	}
}

func TestRuntimeLabelsIncludeExtractionFields(t *testing.T) {
	labels := RuntimeLabels("agent-1", "openclaw", "sandbox-1")
	if labels["app.kubernetes.io/name"] != "shclop-agent-runtime" || labels["app.kubernetes.io/component"] != "agent-runtime" {
		t.Fatalf("unexpected base labels: %#v", labels)
	}
	if labels["shclop.io/agent-id"] != "agent-1" || labels["shclop.io/runtime-flavor"] != "openclaw" || labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("unexpected runtime labels: %#v", labels)
	}
}

func TestRuntimeLabelsSanitizeValues(t *testing.T) {
	labels := RuntimeLabels("agent-1", "openclaw", "sandbox-1")
	if labels["shclop.io/agent-id"] != "agent-1" {
		t.Fatalf("unexpected agent label: %#v", labels)
	}
	ownerLabels := BuildAgentPodSpec(AgentPodRequest{AgentID: "agent-1", OwnerID: "oidc|alice@example.com", Image: "img"}).Labels
	if got := ownerLabels["shclop.io/owner-id"]; got == "" || got == "oidc|alice@example.com" {
		t.Fatalf("expected sanitized owner label, got %#v", ownerLabels)
	}
	if got := ownerLabels["shclop.io/owner-id"]; containsAny(got, "|@") {
		t.Fatalf("expected sanitized owner label, got %q", got)
	}
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		if strings.ContainsRune(s, c) {
			return true
		}
	}
	return false
}

func TestBuildWorkspacePVC(t *testing.T) {
	pvc, err := BuildWorkspacePVC("agent-1", "sandbox-1", "fast-ssd", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pvc.Name != "shclop-workspace-agent-1" {
		t.Fatalf("unexpected pvc name %q", pvc.Name)
	}
	if pvc.Labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("unexpected labels: %#v", pvc.Labels)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
		t.Fatalf("unexpected storage class: %#v", pvc.Spec.StorageClassName)
	}
	if got := pvc.Spec.Resources.Requests["storage"]; got.String() != "10Gi" {
		t.Fatalf("expected default size 10Gi, got %s", got.String())
	}
}

func TestBuildRuntimePod(t *testing.T) {
	pod := BuildRuntimePod(AgentPodSpec{
		Name:                         "agent-1",
		RuntimeClassName:             "kata",
		AutomountServiceAccountToken: false,
		FSGroup:                      10001,
		Container: ContainerSpec{
			Name:                     "runtime",
			Image:                    "image:latest",
			Privileged:               false,
			AllowPrivilegeEscalation: false,
			ReadOnlyRootFilesystem:   true,
			RunAsNonRoot:             true,
			RunAsUser:                10001,
			SeccompProfile:           "RuntimeDefault",
			DropCapabilities:         []string{"ALL"},
			CPU:                      "500m",
			Memory:                   "512Mi",
			Env:                      map[string]string{"B": "2", "A": "1"},
			VolumeMounts:             []VolumeMount{{Name: "workspace", MountPath: "/workspace"}, {Name: "token", MountPath: "/token", ReadOnly: true}},
		},
		Volumes: []VolumeSpec{{Name: "workspace", PVC: "ws"}, {Name: "token", SecretName: "secret"}},
	})
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata" {
		t.Fatalf("unexpected runtime class: %#v", pod.Spec.RuntimeClassName)
	}
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Fatalf("expected false automount")
	}
	if pod.Spec.DNSPolicy != "" {
		t.Fatalf("expected Kubernetes default DNS policy for runtime pods, got %q", pod.Spec.DNSPolicy)
	}
	if pod.Spec.Containers[0].Env[0].Name != "A" || pod.Spec.Containers[0].Env[1].Name != "B" {
		t.Fatalf("env not sorted: %#v", pod.Spec.Containers[0].Env)
	}
	if pod.Spec.Containers[0].SecurityContext == nil || pod.Spec.Containers[0].SecurityContext.Privileged == nil || *pod.Spec.Containers[0].SecurityContext.Privileged {
		t.Fatal("expected hardened security context")
	}
	if len(pod.Spec.Volumes) != 2 || pod.Spec.Volumes[1].Secret == nil || pod.Spec.Volumes[1].Secret.SecretName != "secret" {
		t.Fatalf("unexpected volumes: %#v", pod.Spec.Volumes)
	}
}
