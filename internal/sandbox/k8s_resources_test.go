package sandbox

import "testing"

func TestBuildWorkspacePVCDefaultsAndLabels(t *testing.T) {
	pvc, err := BuildWorkspacePVC("agent-1", "sandbox-1", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pvc.Name != "shclop-workspace-agent-1" {
		t.Fatalf("unexpected name %q", pvc.Name)
	}
	if pvc.Labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("unexpected labels: %#v", pvc.Labels)
	}
	if pvc.Spec.StorageClassName != nil {
		t.Fatalf("expected nil storage class, got %#v", pvc.Spec.StorageClassName)
	}
}

func TestBuildWorkspacePVCRejectsInvalidSize(t *testing.T) {
	if _, err := BuildWorkspacePVC("agent-1", "sandbox-1", "", "not-a-size"); err == nil {
		t.Fatal("expected invalid size error")
	}
}

func TestBuildRuntimePodHardeningAndVolumes(t *testing.T) {
	pod := BuildRuntimePod(AgentPodSpec{
		Name:             "agent-1",
		RuntimeClassName: "kata",
		FSGroup:          10001,
		Container: ContainerSpec{
			Name:             "runtime",
			Image:            "image:latest",
			RunAsUser:        10001,
			Env:              map[string]string{"B": "2", "A": "1"},
			DropCapabilities: []string{"ALL"},
			VolumeMounts:     []VolumeMount{{Name: "workspace", MountPath: "/workspace", ReadOnly: true}, {Name: "runtime-token", MountPath: "/var/run/shclop", ReadOnly: true}},
		},
		Volumes: []VolumeSpec{{Name: "workspace", PVC: "ws"}, {Name: "runtime-token", SecretName: "secret", SecretKey: "token", SecretPath: "token"}},
	})
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata" {
		t.Fatalf("unexpected runtime class: %#v", pod.Spec.RuntimeClassName)
	}
	if pod.Spec.DNSPolicy != "Default" {
		t.Fatalf("expected DNSDefault policy for runtime pods, got %q", pod.Spec.DNSPolicy)
	}
	if pod.Spec.Containers[0].Env[0].Name != "A" || pod.Spec.Containers[0].Env[1].Name != "B" {
		t.Fatalf("env not sorted: %#v", pod.Spec.Containers[0].Env)
	}
	if len(pod.Spec.Volumes) != 2 || pod.Spec.Volumes[1].Secret == nil || pod.Spec.Volumes[1].Secret.SecretName != "secret" || len(pod.Spec.Volumes[1].Secret.Items) != 1 || pod.Spec.Volumes[1].Secret.Items[0].Key != "token" || pod.Spec.Volumes[1].Secret.Items[0].Path != "token" {
		t.Fatalf("unexpected volumes: %#v", pod.Spec.Volumes)
	}
}
