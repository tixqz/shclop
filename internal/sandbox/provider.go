package sandbox

import "fmt"

type Provider interface {
	BuildAgentPod(req AgentRequest) (AgentPodSpec, error)
}

type AgentRequest struct {
	AgentID      string
	OwnerID      string
	Runtime      string
	GatewayURL   string
	SandboxID    string
	SecretRef    SecretRef
	WorkspacePVC string
	CPU          string
	Memory       string
}

type KataProviderConfig struct {
	RuntimeClassName string
	Images           map[string]string
}

type KataProvider struct {
	runtimeClassName string
	images           map[string]string
}

func NewKataProvider(cfg KataProviderConfig) *KataProvider {
	images := make(map[string]string, len(cfg.Images))
	for name, image := range cfg.Images {
		images[name] = image
	}
	return &KataProvider{runtimeClassName: cfg.RuntimeClassName, images: images}
}

func (p *KataProvider) BuildAgentPod(req AgentRequest) (AgentPodSpec, error) {
	image, ok := p.images[req.Runtime]
	if !ok || image == "" {
		return AgentPodSpec{}, fmt.Errorf("unknown runtime %q", req.Runtime)
	}

	return BuildAgentPodSpec(AgentPodRequest{
		AgentID:          req.AgentID,
		OwnerID:          req.OwnerID,
		Image:            image,
		RuntimeClassName: p.runtimeClassName,
		GatewayURL:       req.GatewayURL,
		Runtime:          req.Runtime,
		SandboxID:        req.SandboxID,
		SecretRef:        req.SecretRef,
		WorkspacePVC:     req.WorkspacePVC,
		CPU:              req.CPU,
		Memory:           req.Memory,
	}), nil
}
