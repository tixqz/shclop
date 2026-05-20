package sandbox

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretRef struct {
	Name string
	Key  string
	// EnvName is used by secret-store integrations that inject tokens via env;
	// the default pod path uses SHCLOP_RUNTIME_TOKEN_FILE.
	EnvName   string
	MountPath string
}

type RuntimeSecretStore interface {
	BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error)
	DeleteRuntimeTokenName(agentID string) string
}

type KubernetesSecretStore struct {
	Namespace string
}

func (s KubernetesSecretStore) BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error) {
	agentID = strings.TrimSpace(agentID)
	token = strings.TrimSpace(token)
	if agentID == "" {
		return SecretRef{}, nil, fmt.Errorf("agentID is required")
	}
	if token == "" {
		return SecretRef{}, nil, fmt.Errorf("token is required")
	}

	name := s.DeleteRuntimeTokenName(agentID)
	return SecretRef{
			Name:      name,
			Key:       "token",
			EnvName:   "SHCLOP_RUNTIME_TOKEN",
			MountPath: "/var/run/shclop/token",
		}, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: s.Namespace,
				Labels:    RuntimeLabels(agentID, "", ""),
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"token": token,
			},
		}, nil
}

func (s KubernetesSecretStore) DeleteRuntimeTokenName(agentID string) string {
	return "shclop-runtime-token-" + strings.TrimSpace(agentID)
}
