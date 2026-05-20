package sandbox

import (
	"testing"
)

func TestNetworkPolicySpecFromConfigParsesCIDRs(t *testing.T) {
	spec := NetworkPolicySpecFromConfig(true, "restricted", "10.0.0.0/8, 192.168.0.0/16, ,")
	if !spec.Enabled || spec.Mode != NetworkPolicyRestricted {
		t.Fatalf("unexpected spec: %#v", spec)
	}
	if len(spec.AllowedEgress) != 2 {
		t.Fatalf("expected 2 egress rules, got %#v", spec.AllowedEgress)
	}
	if spec.AllowedEgress[0].Name != "custom-1" || spec.AllowedEgress[0].CIDR != "10.0.0.0/8" || len(spec.AllowedEgress[0].Ports) != 1 || spec.AllowedEgress[0].Ports[0] != 443 {
		t.Fatalf("unexpected first egress: %#v", spec.AllowedEgress[0])
	}
	if spec.AllowedEgress[1].Name != "custom-2" || spec.AllowedEgress[1].CIDR != "192.168.0.0/16" {
		t.Fatalf("unexpected second egress: %#v", spec.AllowedEgress[1])
	}
}

func TestBuildRuntimeNetworkPolicyRestricted(t *testing.T) {
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", NetworkPolicySpecFromConfig(true, "restricted", "10.0.0.0/8"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy == nil {
		t.Fatal("expected policy")
	}
	if policy.Name != "shclop-runtime-netpol-agent-1" {
		t.Fatalf("unexpected name %q", policy.Name)
	}
	if policy.Labels["shclop.io/agent-id"] != "agent-1" || policy.Labels["shclop.io/sandbox-id"] != "sandbox-1" {
		t.Fatalf("unexpected labels: %#v", policy.Labels)
	}
	if len(policy.Spec.Egress) == 0 {
		t.Fatal("expected at least one egress rule")
	}
	found443 := false
	for _, rule := range policy.Spec.Egress {
		for _, port := range rule.Ports {
			if port.Port != nil && port.Port.IntVal == 443 {
				found443 = true
			}
		}
	}
	if !found443 {
		t.Fatalf("expected 443 port rule, got %#v", policy.Spec.Egress)
	}
}

func TestBuildRuntimeNetworkPolicyDisabled(t *testing.T) {
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", NetworkPolicySpec{Enabled: false, Mode: NetworkPolicyDisabled})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy != nil {
		t.Fatalf("expected nil policy, got %#v", policy)
	}
}

func TestBuildRuntimeNetworkPolicyNoCIDRsKeepsEgressEmpty(t *testing.T) {
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy == nil || policy.Spec.PolicyTypes[0] != "Egress" {
		t.Fatalf("unexpected policy: %#v", policy)
	}
	if len(policy.Spec.Egress) != 0 {
		t.Fatalf("expected empty egress list, got %#v", policy.Spec.Egress)
	}
}

func TestBuildRuntimeNetworkPolicyAllowsBackendAndVault(t *testing.T) {
	policy, err := BuildRuntimeNetworkPolicy("agent-1", "sandbox-1", NetworkPolicySpec{Enabled: true, Mode: NetworkPolicyRestricted, AllowBackend: true, AllowVault: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy == nil {
		t.Fatal("expected policy")
	}
	if len(policy.Spec.Egress) < 2 {
		t.Fatalf("expected backend and vault egress rules, got %#v", policy.Spec.Egress)
	}
	if policy.Spec.Egress[0].To != nil {
		t.Fatalf("expected backend rule to allow all destinations, got %#v", policy.Spec.Egress[0].To)
	}
	if policy.Spec.Egress[0].Ports[2].Port == nil || policy.Spec.Egress[0].Ports[2].Port.IntVal != 8080 {
		t.Fatalf("expected 8080 backend port, got %#v", policy.Spec.Egress[0].Ports)
	}
	if policy.Spec.Egress[1].Ports[0].Port == nil || policy.Spec.Egress[1].Ports[0].Port.IntVal != 8200 {
		t.Fatalf("expected vault 8200 port, got %#v", policy.Spec.Egress[1].Ports)
	}
}
