package checks

import "time"

// ComplianceCheck defines the interface for all compliance checks
type ComplianceCheck interface {
	ID() string
	Description() string
	Execute(binaryPath string) CheckResult
}

// CheckResult represents the result of a compliance check
type CheckResult struct {
	ID          string                 `json:"check_id"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"` // "pass" | "fail"
	Details     string                 `json:"details"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ComplianceReport represents the complete report of all compliance checks
type ComplianceReport struct {
	Timestamp    time.Time     `json:"timestamp"`
	BinaryPath   string        `json:"binary_path"`
	BinaryHash   string        `json:"binary_hash"`
	TotalChecks  int           `json:"total_checks"`
	PassedChecks int           `json:"passed_checks"`
	FailedChecks int           `json:"failed_checks"`
	Results      []CheckResult `json:"results"`
	SBOMPath     string        `json:"sbom_path,omitempty"`
}