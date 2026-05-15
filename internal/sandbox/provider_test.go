package sandbox

import "testing"

func TestKataProviderBuildsAgentPodFromRuntimeImageCatalog(t *testing.T) {
	provider := NewKataProvider(KataProviderConfig{
		RuntimeClassName: "kata",
		Images: map[string]string{
			"openclaw": "shclop-runtime-openclaw:latest",
		},
	})

	pod, err := provider.BuildAgentPod(AgentRequest{
		AgentID:      "agent-1",
		OwnerID:      "user-1",
		Runtime:      "openclaw",
		WorkspacePVC: "agent-1-workspace",
		CPU:          "1000m",
		Memory:       "1Gi",
	})
	if err != nil {
		t.Fatalf("build agent pod: %v", err)
	}

	if pod.RuntimeClassName != "kata" {
		t.Fatalf("expected kata runtime class, got %q", pod.RuntimeClassName)
	}
	if pod.Container.Image != "shclop-runtime-openclaw:latest" {
		t.Fatalf("expected openclaw image, got %q", pod.Container.Image)
	}
	if pod.AutomountServiceAccountToken {
		t.Fatal("expected runtime pod service account token automount to be disabled")
	}
}

func TestKataProviderRejectsUnknownRuntime(t *testing.T) {
	provider := NewKataProvider(KataProviderConfig{
		RuntimeClassName: "kata",
		Images:           map[string]string{"openclaw": "shclop-runtime-openclaw:latest"},
	})

	_, err := provider.BuildAgentPod(AgentRequest{Runtime: "missing"})
	if err == nil {
		t.Fatal("expected unknown runtime error")
	}
}
