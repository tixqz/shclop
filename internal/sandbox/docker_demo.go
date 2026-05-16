package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type StartRequest struct {
	AgentID      string
	OwnerID      string
	Runtime      string
	RuntimeToken string
}

type RuntimeLease struct {
	AgentID    string
	Provider   string
	Runtime    string
	ExternalID string
}

type RuntimeProvider interface {
	Start(ctx context.Context, request StartRequest) (RuntimeLease, error)
	Stop(ctx context.Context, agentID string) error
}

type CommandRunner interface {
	Run(ctx context.Context, args ...string) error
}

type DockerCLI struct{}

func (DockerCLI) Run(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, "docker", args...).Run()
}

type DockerDemoProvider struct {
	Runner      CommandRunner
	GatewayURL  string
	ImagePrefix string
}

func (p DockerDemoProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
	if strings.TrimSpace(request.AgentID) == "" || strings.TrimSpace(request.RuntimeToken) == "" {
		return RuntimeLease{}, errors.New("agent id and runtime token are required")
	}
	runtime := normalizeRuntime(request.Runtime)
	if !isKnownRuntime(runtime) {
		return RuntimeLease{}, fmt.Errorf("unsupported runtime %q", request.Runtime)
	}
	runner := p.Runner
	if runner == nil {
		runner = DockerCLI{}
	}
	gatewayURL := p.GatewayURL
	if gatewayURL == "" {
		gatewayURL = "ws://host.docker.internal:8080/runtime/ws"
	}
	imagePrefix := p.ImagePrefix
	if imagePrefix == "" {
		imagePrefix = "shclop-runtime"
	}
	containerName := "shclop-agent-" + request.AgentID
	image := imagePrefix + "-" + runtime + ":latest"
	args := []string{
		"run", "--rm", "--detach",
		"--name", containerName,
		"--add-host=host.docker.internal:host-gateway",
		"-e", "SHCLOP_GATEWAY_URL=" + gatewayURL,
		"-e", "SHCLOP_AGENT_ID=" + request.AgentID,
		"-e", "SHCLOP_RUNTIME_TOKEN=" + request.RuntimeToken,
		"-e", "SHCLOP_AGENT_FLAVOR=" + runtime,
		image,
	}
	if err := runner.Run(ctx, args...); err != nil {
		return RuntimeLease{}, err
	}
	return RuntimeLease{AgentID: request.AgentID, Provider: "docker-demo", Runtime: runtime, ExternalID: containerName}, nil
}

func (p DockerDemoProvider) Stop(ctx context.Context, agentID string) error {
	runner := p.Runner
	if runner == nil {
		runner = DockerCLI{}
	}
	return runner.Run(ctx, "rm", "-f", "shclop-agent-"+agentID)
}

func normalizeRuntime(runtime string) string {
	runtime = strings.TrimSpace(strings.ToLower(runtime))
	if runtime == "" {
		return "openclaw"
	}
	return runtime
}

func isKnownRuntime(runtime string) bool {
	switch runtime {
	case "openclaw", "nanoclaw", "nemoclaw":
		return true
	default:
		return false
	}
}
