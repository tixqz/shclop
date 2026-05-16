package sandbox

import "context"

type MockRuntimeProvider struct{}

func (MockRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeLease{}, err
	}
	runtime := normalizeRuntime(request.Runtime)
	return RuntimeLease{AgentID: request.AgentID, Provider: "mock", Runtime: runtime, ExternalID: "mock-" + request.AgentID}, nil
}

func (MockRuntimeProvider) Stop(ctx context.Context, agentID string) error {
	return ctx.Err()
}
