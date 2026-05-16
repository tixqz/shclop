package sandbox

import (
	"context"
	"reflect"
	"testing"
)

func TestDockerDemoProviderBuildsLocalRuntimeCommand(t *testing.T) {
	runner := &recordingRunner{}
	provider := DockerDemoProvider{
		Runner:      runner,
		GatewayURL:  "ws://host.docker.internal:8080/runtime/ws",
		ImagePrefix: "shclop-runtime",
	}

	lease, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", RuntimeToken: "secret", Runtime: "openclaw"})
	if err != nil {
		t.Fatal(err)
	}
	if lease.AgentID != "agent-1" || lease.Provider != "docker-demo" || lease.Runtime != "openclaw" {
		t.Fatalf("unexpected lease: %#v", lease)
	}
	want := []string{
		"run", "--rm", "--detach",
		"--name", "shclop-agent-agent-1",
		"--add-host=host.docker.internal:host-gateway",
		"-e", "SHCLOP_GATEWAY_URL=ws://host.docker.internal:8080/runtime/ws",
		"-e", "SHCLOP_AGENT_ID=agent-1",
		"-e", "SHCLOP_RUNTIME_TOKEN=secret",
		"-e", "SHCLOP_AGENT_FLAVOR=openclaw",
		"shclop-runtime-openclaw:latest",
	}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected docker args:\nwant %#v\n got %#v", want, runner.args)
	}
}

func TestDockerDemoProviderRejectsUnknownRuntime(t *testing.T) {
	provider := DockerDemoProvider{Runner: &recordingRunner{}}
	if _, err := provider.Start(context.Background(), StartRequest{AgentID: "agent-1", RuntimeToken: "secret", Runtime: "unknown"}); err == nil {
		t.Fatal("expected unknown runtime error")
	}
}

type recordingRunner struct{ args []string }

func (r *recordingRunner) Run(ctx context.Context, args ...string) error {
	r.args = append([]string(nil), args...)
	return nil
}
