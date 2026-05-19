package security

type PolicyMode string

const (
	ModeEnforce         PolicyMode = "enforce"
	ModeWarn            PolicyMode = "warn"
	ModeDisabledDevOnly PolicyMode = "disabled_dev_only"
)

type Policy struct {
	Mode    PolicyMode `json:"mode"`
	Version int        `json:"version"`
}

func DefaultPolicy() Policy {
	return Policy{Mode: ModeEnforce, Version: 1}
}

func EvaluatePolicy(policy Policy, result ScanResult) Decision {
	switch policy.Mode {
	case ModeWarn, ModeDisabledDevOnly:
		if len(result.Findings) == 0 && result.RiskLevel == RiskNone {
			return DecisionApproved
		}
		return DecisionWarnOnlyAllowed
	default:
		if result.RiskLevel == RiskHigh || result.RiskLevel == RiskCritical {
			return DecisionRejected
		}
		if result.RiskLevel == RiskMedium {
			return DecisionPendingApproval
		}
		if result.RiskLevel == RiskLow {
			return DecisionApprovedWithWarnings
		}
		return DecisionApproved
	}
}

func DecisionAllowsUse(decision Decision) bool {
	return decision == DecisionApproved || decision == DecisionApprovedWithWarnings || decision == DecisionWarnOnlyAllowed
}
