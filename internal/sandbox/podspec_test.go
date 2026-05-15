package sandbox

import "testing"

func TestAgentPodSpecUsesKataAndHardening(t *testing.T) {
	spec := BuildAgentPodSpec(AgentPodRequest{
		AgentID:          "agent-1",
		OwnerID:          "user-1",
		Image:            "shclop-runtime-openclaw:latest",
		RuntimeClassName: "kata",
		WorkspacePVC:     "agent-1-workspace",
		CPU:              "1000m",
		Memory:           "1Gi",
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
}
