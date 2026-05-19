package security

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Decision string

const (
	DecisionApproved             Decision = "approved"
	DecisionApprovedWithWarnings Decision = "approved_with_warnings"
	DecisionPendingApproval      Decision = "pending_approval"
	DecisionRejected             Decision = "rejected"
	DecisionWarnOnlyAllowed      Decision = "warn_only_allowed"
)

type Finding struct {
	RuleID         string    `json:"rule_id"`
	Severity       RiskLevel `json:"severity"`
	Message        string    `json:"message"`
	Evidence       string    `json:"evidence,omitempty"`
	Recommendation string    `json:"recommendation,omitempty"`
}

type ScanInput struct {
	TargetType string
	Text       string
}

type ScanResult struct {
	RiskLevel RiskLevel `json:"risk_level"`
	Decision  Decision  `json:"decision"`
	Findings  []Finding `json:"findings"`
}

type DeterministicScanner struct{}

func NewDeterministicScanner() DeterministicScanner { return DeterministicScanner{} }

func (DeterministicScanner) Scan(input ScanInput) ScanResult {
	lower := strings.ToLower(input.Text)
	findings := make([]Finding, 0)
	if containsAny(lower, "exfiltrate", "steal secrets", "dump secrets", "send secrets", "vault token") {
		findings = append(findings, Finding{RuleID: "secrets.exfiltration", Severity: RiskCritical, Message: "content asks to expose or exfiltrate secrets", Evidence: evidence(input.Text), Recommendation: "remove secret access/exfiltration instructions"})
	}
	if containsAny(lower, "ignore policy", "bypass policy", "disable audit", "skip approval") {
		findings = append(findings, Finding{RuleID: "policy.bypass", Severity: RiskHigh, Message: "content asks to bypass policy or approval", Evidence: evidence(input.Text), Recommendation: "remove policy bypass instructions"})
	}
	if containsAny(lower, "unrestricted network", "any external url", "private cidr", "169.254.169.254") {
		findings = append(findings, Finding{RuleID: "network.unrestricted", Severity: RiskHigh, Message: "content requests unsafe network access", Evidence: evidence(input.Text), Recommendation: "scope egress to approved destinations"})
	}
	if containsAny(lower, "prompt injection", "developer message", "system prompt") {
		findings = append(findings, Finding{RuleID: "prompt.injection", Severity: RiskMedium, Message: "content references prompt-injection sensitive behavior", Evidence: evidence(input.Text), Recommendation: "clarify safe handling of instructions"})
	}
	risk := maxRisk(findings)
	return ScanResult{RiskLevel: risk, Decision: EvaluatePolicy(Policy{Mode: ModeEnforce, Version: 1}, ScanResult{RiskLevel: risk, Findings: findings}), Findings: findings}
}

func ContentDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func evidence(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 160 {
		return trimmed
	}
	return trimmed[:160]
}

func maxRisk(findings []Finding) RiskLevel {
	risk := RiskNone
	for _, finding := range findings {
		if riskRank(finding.Severity) > riskRank(risk) {
			risk = finding.Severity
		}
	}
	return risk
}

func riskRank(risk RiskLevel) int {
	switch risk {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}
