package sandbox

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type NetworkPolicyMode string

const (
	NetworkPolicyDisabled   NetworkPolicyMode = "disabled"
	NetworkPolicyRestricted NetworkPolicyMode = "restricted"
	NetworkPolicyCustom     NetworkPolicyMode = "custom"
)

type EgressRule struct {
	Name    string
	CIDR    string
	DNSName string
	Ports   []int32
}

type NetworkPolicySpec struct {
	Enabled       bool
	Mode          NetworkPolicyMode
	AllowBackend  bool
	AllowVault    bool
	AllowedEgress []EgressRule
}

func NetworkPolicySpecFromConfig(enabled bool, mode string, cidrs string) NetworkPolicySpec {
	spec := NetworkPolicySpec{Enabled: enabled, Mode: NetworkPolicyMode(strings.TrimSpace(mode)), AllowBackend: true, AllowVault: true}
	if spec.Mode == "" {
		spec.Mode = NetworkPolicyRestricted
	}
	index := 0
	for _, raw := range strings.Split(cidrs, ",") {
		cidr := strings.TrimSpace(raw)
		if cidr == "" {
			continue
		}
		index++
		spec.AllowedEgress = append(spec.AllowedEgress, EgressRule{Name: "custom-" + strconv.Itoa(index), CIDR: cidr, Ports: []int32{443}})
	}
	return spec
}

func RuntimeLabels(agentID, runtime, sandboxID string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":      "shclop-agent-runtime",
		"app.kubernetes.io/component": "agent-runtime",
	}
	if v := labelValue(agentID); v != "" {
		labels["shclop.io/agent-id"] = v
	}
	if v := labelValue(runtime); v != "" {
		labels["shclop.io/runtime-flavor"] = v
	}
	if v := labelValue(sandboxID); v != "" {
		labels["shclop.io/sandbox-id"] = v
	}
	return labels
}

func BuildRuntimeNetworkPolicy(agentID, sandboxID string, spec NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {
	if !spec.Enabled || spec.Mode == NetworkPolicyDisabled {
		return nil, nil
	}

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "shclop-runtime-netpol-" + agentID,
			Labels: RuntimeLabels(agentID, "", sandboxID),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"shclop.io/agent-id": labelValue(agentID)}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
	if spec.AllowBackend {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: portOrNil(53), Protocol: udpProtocol()}, {Port: portOrNil(53), Protocol: tcpProtocol()}, {Port: portOrNil(80), Protocol: tcpProtocol()}, {Port: portOrNil(443), Protocol: tcpProtocol()}, {Port: portOrNil(4000), Protocol: tcpProtocol()}, {Port: portOrNil(8080), Protocol: tcpProtocol()}},
		})
	}
	if spec.AllowVault {
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			Ports: []networkingv1.NetworkPolicyPort{{Port: portOrNil(8200), Protocol: tcpProtocol()}},
		})
	}

	for _, rule := range spec.AllowedEgress {
		if strings.TrimSpace(rule.CIDR) == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(strings.TrimSpace(rule.CIDR)); err != nil {
			return nil, fmt.Errorf("invalid cidr %q: %w", rule.CIDR, err)
		}
		rulePorts := make([]networkingv1.NetworkPolicyPort, 0, len(rule.Ports))
		for _, port := range rule.Ports {
			p := port
			proto := corev1.ProtocolTCP
			rulePorts = append(rulePorts, networkingv1.NetworkPolicyPort{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: p}, Protocol: &proto})
		}
		policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To:    []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: strings.TrimSpace(rule.CIDR)}}},
			Ports: rulePorts,
		})
	}

	return policy, nil
}

func tcpProtocol() *corev1.Protocol {
	p := corev1.ProtocolTCP
	return &p
}

func udpProtocol() *corev1.Protocol {
	p := corev1.ProtocolUDP
	return &p
}

func portOrNil(port int32) *intstr.IntOrString {
	p := intstr.FromInt(int(port))
	return &p
}

func labelValue(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	buf := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			buf = append(buf, r)
		} else if len(buf) == 0 || buf[len(buf)-1] != '-' {
			buf = append(buf, '-')
		}
	}
	value = strings.Trim(string(buf), "-.")
	if value == "" {
		return ""
	}
	if len(value) > 63 {
		value = strings.TrimRightFunc(value[:63], func(r rune) bool { return r == '-' || r == '.' || unicode.IsSpace(r) })
	}
	return value
}
