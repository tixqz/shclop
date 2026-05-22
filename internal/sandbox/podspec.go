package sandbox

import "path/filepath"

type AgentPodRequest struct {
	AgentID          string
	OwnerID          string
	Image            string
	RuntimeClassName string
	GatewayURL       string
	Runtime          string
	SandboxID        string
	SecretRef        SecretRef
	WorkspacePVC     string
	CPU              string
	Memory           string
	// LLM configuration
	LLMGatewayBaseURL   string
	LLMModel            string
	LLMGatewaySecretRef *SecretKeyRef
}

type SecretKeyRef struct {
	Name   string
	Key    string
	EnvVar string // Target env var name
}

type AgentPodSpec struct {
	Name                         string
	Labels                       map[string]string
	RuntimeClassName             string
	AutomountServiceAccountToken bool
	HostNetwork                  bool
	HostPID                      bool
	HostIPC                      bool
	FSGroup                      int64
	Container                    ContainerSpec
	Volumes                      []VolumeSpec
}

type ContainerSpec struct {
	Name                     string
	Image                    string
	Privileged               bool
	AllowPrivilegeEscalation bool
	ReadOnlyRootFilesystem   bool
	RunAsNonRoot             bool
	RunAsUser                int64
	SeccompProfile           string
	DropCapabilities         []string
	CPU                      string
	Memory                   string
	Env                      map[string]string
	EnvFrom                  []EnvFromSource // SecretKeyRef entries
	VolumeMounts             []VolumeMount
}

type EnvFromSource struct {
	SecretName string
	SecretKey  string
	EnvVar     string // Target environment variable name
}

type VolumeSpec struct {
	Name       string
	PVC        string
	SecretName string
	SecretKey  string
	SecretPath string
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

func BuildAgentPodSpec(req AgentPodRequest) AgentPodSpec {
	env := map[string]string{
		"SHCLOP_GATEWAY_URL":        req.GatewayURL,
		"SHCLOP_AGENT_ID":           req.AgentID,
		"SHCLOP_AGENT_FLAVOR":       req.Runtime,
		"SHCLOP_RUNTIME_TOKEN_FILE": req.SecretRef.MountPath,
	}

	// Add LLM environment variables
	if req.LLMGatewayBaseURL != "" {
		env["LLM_GATEWAY_BASE_URL"] = req.LLMGatewayBaseURL
	}
	if req.LLMModel != "" {
		env["LLM_GATEWAY_MODEL"] = req.LLMModel
	}

	var envFrom []EnvFromSource
	if req.LLMGatewaySecretRef != nil {
		envFrom = append(envFrom, EnvFromSource{
			SecretName: req.LLMGatewaySecretRef.Name,
			SecretKey:  req.LLMGatewaySecretRef.Key,
			EnvVar:     req.LLMGatewaySecretRef.EnvVar,
		})
	}

	spec := AgentPodSpec{
		Name:             "agent-" + req.AgentID,
		RuntimeClassName: req.RuntimeClassName,
		Labels: func() map[string]string {
			labels := RuntimeLabels(req.AgentID, req.Runtime, req.SandboxID)
			if v := labelValue(req.OwnerID); v != "" {
				labels["shclop.io/owner-id"] = v
			}
			return labels
		}(),
		AutomountServiceAccountToken: false,
		HostNetwork:                  false,
		HostPID:                      false,
		HostIPC:                      false,
		FSGroup:                      10001,
		Container: ContainerSpec{
			Name:                     "runtime",
			Image:                    req.Image,
			Privileged:               false,
			AllowPrivilegeEscalation: false,
			ReadOnlyRootFilesystem:   true,
			RunAsNonRoot:             true,
			RunAsUser:                10001,
			SeccompProfile:           "RuntimeDefault",
			DropCapabilities:         []string{"ALL"},
			CPU:                      req.CPU,
			Memory:                   req.Memory,
			Env:                      env,
			EnvFrom:                  envFrom,
			VolumeMounts: []VolumeMount{
				{Name: "workspace", MountPath: "/workspace"},
				{Name: "memory", MountPath: "/memory"},
			},
		},
		Volumes: []VolumeSpec{
			{Name: "workspace", PVC: req.WorkspacePVC},
			{Name: "memory", PVC: req.WorkspacePVC},
		},
	}

	if req.SecretRef.Name != "" && req.SecretRef.MountPath != "" {
		mountDir := filepath.Dir(req.SecretRef.MountPath)
		mountBase := filepath.Base(req.SecretRef.MountPath)
		secretKey := req.SecretRef.Key
		if secretKey == "" {
			secretKey = mountBase
		}
		spec.Volumes = append(spec.Volumes, VolumeSpec{Name: "runtime-token", SecretName: req.SecretRef.Name, SecretKey: secretKey, SecretPath: mountBase})
		spec.Container.VolumeMounts = append(spec.Container.VolumeMounts, VolumeMount{Name: "runtime-token", MountPath: mountDir, ReadOnly: true})
	}

	return spec
}
