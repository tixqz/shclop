package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesRuntimeProviderConfig struct {
	Namespace         string
	GatewayURL        string
	RuntimeClassName  string
	Images            map[string]string
	WorkspaceSize     string
	StorageClassName  string
	WorkspacePolicy   string
	SecretStore       string
	NetworkPolicySpec NetworkPolicySpec
}

type KubernetesRuntimeProvider struct {
	cfg         KubernetesRuntimeProviderConfig
	client      kubernetes.Interface
	secretStore RuntimeSecretStore
}

func NewKubernetesRuntimeProvider(cfg KubernetesRuntimeProviderConfig) (*KubernetesRuntimeProvider, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.WorkspaceSize == "" {
		cfg.WorkspaceSize = "10Gi"
	}
	if cfg.WorkspacePolicy == "" {
		cfg.WorkspacePolicy = "delete"
	}
	if cfg.SecretStore == "" {
		cfg.SecretStore = "kubernetes"
	}
	if cfg.SecretStore != "kubernetes" {
		return nil, fmt.Errorf("unsupported secret store %q", cfg.SecretStore)
	}

	images := make(map[string]string, len(cfg.Images))
	for name, image := range cfg.Images {
		images[name] = image
	}
	cfg.Images = images

	client, err := buildKubernetesClient()
	if err != nil {
		return nil, err
	}

	return &KubernetesRuntimeProvider{
		cfg:         cfg,
		client:      client,
		secretStore: KubernetesSecretStore{Namespace: cfg.Namespace},
	}, nil
}

func (p *KubernetesRuntimeProvider) Config() KubernetesRuntimeProviderConfig {
	return p.cfg
}

func (p *KubernetesRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
	if strings.TrimSpace(request.AgentID) == "" || strings.TrimSpace(request.RuntimeToken) == "" {
		return RuntimeLease{}, errors.New("agent id and runtime token are required")
	}
	runtime := normalizeRuntime(request.Runtime)
	if !isKnownRuntime(runtime) {
		return RuntimeLease{}, fmt.Errorf("unsupported runtime %q", request.Runtime)
	}
	image, ok := p.cfg.Images[runtime]
	if !ok || strings.TrimSpace(image) == "" {
		return RuntimeLease{}, fmt.Errorf("missing runtime image for %q", runtime)
	}
	if p.client == nil {
		return RuntimeLease{}, errors.New("kubernetes client is not configured")
	}

	sandboxID := "sandbox-" + request.AgentID
	secretRef, secret, err := p.secretStore.BuildRuntimeTokenSecret(request.AgentID, request.RuntimeToken)
	if err != nil {
		return RuntimeLease{}, err
	}
	pvc, err := BuildWorkspacePVC(request.AgentID, sandboxID, p.cfg.StorageClassName, p.cfg.WorkspaceSize)
	if err != nil {
		return RuntimeLease{}, err
	}
	policy, err := BuildRuntimeNetworkPolicy(request.AgentID, sandboxID, p.cfg.NetworkPolicySpec)
	if err != nil {
		return RuntimeLease{}, err
	}
	podSpec := BuildAgentPodSpec(AgentPodRequest{
		AgentID:          request.AgentID,
		OwnerID:          request.OwnerID,
		Image:            image,
		RuntimeClassName: p.cfg.RuntimeClassName,
		GatewayURL:       p.cfg.GatewayURL,
		Runtime:          runtime,
		SandboxID:        sandboxID,
		SecretRef:        secretRef,
		WorkspacePVC:     pvc.Name,
	})
	pod := BuildRuntimePod(podSpec)
	pod.Namespace = p.cfg.Namespace
	if secret != nil {
		secret.Namespace = p.cfg.Namespace
	}
	pvc.Namespace = p.cfg.Namespace
	if policy != nil {
		policy.Namespace = p.cfg.Namespace
	}

	if err := p.createOrUpdateSecret(ctx, secret); err != nil {
		return RuntimeLease{}, err
	}
	if err := p.createOrUpdatePVC(ctx, pvc); err != nil {
		return RuntimeLease{}, err
	}
	if err := p.createOrUpdateNetworkPolicy(ctx, policy); err != nil {
		return RuntimeLease{}, err
	}
	if err := p.createOrUpdatePod(ctx, pod); err != nil {
		return RuntimeLease{}, err
	}

	return RuntimeLease{AgentID: request.AgentID, Provider: "kubernetes", Runtime: runtime, ExternalID: pod.Name}, nil
}

func (p *KubernetesRuntimeProvider) Stop(ctx context.Context, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return errors.New("agent id is required")
	}
	if p.client == nil {
		return errors.New("kubernetes client is not configured")
	}
	policyName := "shclop-runtime-netpol-" + agentID
	secretName := p.secretStore.DeleteRuntimeTokenName(agentID)
	pvcName := "shclop-workspace-" + agentID
	podName := "agent-" + agentID
	if err := deleteIgnoreNotFound(ctx, p.client.CoreV1().Pods(p.cfg.Namespace).Delete, podName); err != nil {
		return err
	}
	if err := deleteIgnoreNotFound(ctx, p.client.NetworkingV1().NetworkPolicies(p.cfg.Namespace).Delete, policyName); err != nil {
		return err
	}
	if err := deleteIgnoreNotFound(ctx, p.client.CoreV1().Secrets(p.cfg.Namespace).Delete, secretName); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(p.cfg.WorkspacePolicy), "delete") {
		if err := deleteIgnoreNotFound(ctx, p.client.CoreV1().PersistentVolumeClaims(p.cfg.Namespace).Delete, pvcName); err != nil {
			return err
		}
	}
	return nil
}

type deleteFunc func(context.Context, string, metav1.DeleteOptions) error

func deleteIgnoreNotFound(ctx context.Context, deleteFn deleteFunc, name string) error {
	if err := deleteFn(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func buildKubernetesClient() (kubernetes.Interface, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		client, clientErr := kubernetes.NewForConfig(cfg)
		if clientErr == nil {
			return client, nil
		}
	}
	if kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG")); kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("build kubernetes client from kubeconfig: %w", err)
		}
		client, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("create kubernetes client: %w", err)
		}
		return client, nil
	}
	return nil, errors.New("unable to configure kubernetes client from in-cluster config or KUBECONFIG")
}

func (p *KubernetesRuntimeProvider) namespace() string { return p.cfg.Namespace }

func (p *KubernetesRuntimeProvider) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	client := p.client.CoreV1().Secrets(secret.Namespace)
	_, err := client.Create(ctx, secret, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing.StringData = secret.StringData
	existing.Data = secret.Data
	existing.Labels = secret.Labels
	existing.Type = secret.Type
	_, err = client.Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (p *KubernetesRuntimeProvider) createOrUpdatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	client := p.client.CoreV1().PersistentVolumeClaims(pvc.Namespace)
	_, err := client.Create(ctx, pvc, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing.Labels = pvc.Labels
	_, err = client.Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (p *KubernetesRuntimeProvider) createOrUpdateNetworkPolicy(ctx context.Context, policy *networkingv1.NetworkPolicy) error {
	if policy == nil {
		return nil
	}
	client := p.client.NetworkingV1().NetworkPolicies(policy.Namespace)
	_, err := client.Create(ctx, policy, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, policy.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing.Labels = policy.Labels
	existing.Spec = policy.Spec
	_, err = client.Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (p *KubernetesRuntimeProvider) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {
	client := p.client.CoreV1().Pods(pod.Namespace)
	_, err := client.Create(ctx, pod, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	existing, err := client.Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	existing.Labels = pod.Labels
	_, err = client.Update(ctx, existing, metav1.UpdateOptions{})
	return err
}
