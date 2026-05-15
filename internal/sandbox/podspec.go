package sandbox

type AgentPodRequest struct {
	AgentID          string
	OwnerID          string
	Image            string
	RuntimeClassName string
	WorkspacePVC     string
	CPU              string
	Memory           string
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
	VolumeMounts             []VolumeMount
}

type VolumeSpec struct {
	Name string
	PVC  string
}

type VolumeMount struct {
	Name      string
	MountPath string
}

func BuildAgentPodSpec(req AgentPodRequest) AgentPodSpec {
	return AgentPodSpec{
		Name:             "agent-" + req.AgentID,
		RuntimeClassName: req.RuntimeClassName,
		Labels: map[string]string{
			"app.kubernetes.io/name":      "shclop-agent-runtime",
			"app.kubernetes.io/component": "agent-runtime",
			"shclop.io/agent-id":          req.AgentID,
			"shclop.io/owner-id":          req.OwnerID,
		},
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
}
