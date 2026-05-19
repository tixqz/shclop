package security

import "testing"

func TestPolicyEnforceRejectsHighRisk(t *testing.T) {
	decision := EvaluatePolicy(Policy{Mode: ModeEnforce, Version: 1}, ScanResult{RiskLevel: RiskHigh, Findings: []Finding{{RuleID: "network.unrestricted", Severity: RiskHigh}}})
	if decision != DecisionRejected {
		t.Fatalf("expected rejected, got %s", decision)
	}
}

func TestPolicyWarnAllowsWithWarnings(t *testing.T) {
	decision := EvaluatePolicy(Policy{Mode: ModeWarn, Version: 1}, ScanResult{RiskLevel: RiskHigh, Findings: []Finding{{RuleID: "prompt.bypass", Severity: RiskHigh}}})
	if decision != DecisionWarnOnlyAllowed {
		t.Fatalf("expected warn-only allowed, got %s", decision)
	}
}

func TestPolicyEnforceApprovesLowRiskWithWarnings(t *testing.T) {
	decision := EvaluatePolicy(Policy{Mode: ModeEnforce, Version: 1}, ScanResult{RiskLevel: RiskLow, Findings: []Finding{{RuleID: "audit.suspicious", Severity: RiskLow}}})
	if decision != DecisionApprovedWithWarnings {
		t.Fatalf("expected approved with warnings, got %s", decision)
	}
}
