package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// LBPolicy
// =============================================================================

// LBPolicy represents a load balancing strategy
type LBPolicy string

const (
	LBPolicyRoundRobin LBPolicy = "round_robin"
	LBPolicyLeastConn  LBPolicy = "least_conn"
	LBPolicyIPHash     LBPolicy = "ip_hash"
	LBPolicyFirst      LBPolicy = "first"
	LBPolicyRandom     LBPolicy = "random"
)

var allLBPolicies = []LBPolicy{
	LBPolicyRoundRobin, LBPolicyLeastConn, LBPolicyIPHash,
	LBPolicyFirst, LBPolicyRandom,
}

var lbPolicyLabels = map[LBPolicy]string{
	LBPolicyRoundRobin: "Round Robin",
	LBPolicyLeastConn:  "Least Connections",
	LBPolicyIPHash:     "IP Hash (Sticky Sessions)",
	LBPolicyFirst:      "First Available",
	LBPolicyRandom:     "Random",
}

func (p LBPolicy) String() string {
	return string(p)
}

func (p LBPolicy) Label() string {
	return enumtypes.Label(p, lbPolicyLabels, "Unknown")
}

func (p LBPolicy) IsValid() bool {
	return enumtypes.IsValid(p, allLBPolicies...)
}

func (p *LBPolicy) Scan(value interface{}) error {
	if value == nil {
		*p = LBPolicyRoundRobin
		return nil
	}

	return enumtypes.ScanString(p, value)
}

func (p LBPolicy) Value() (driver.Value, error) {
	return enumtypes.ValueString(p)
}

func ParseLBPolicy(s string) (LBPolicy, error) {
	policy := LBPolicy(s)
	if !policy.IsValid() {
		return LBPolicyRoundRobin, fmt.Errorf("invalid load balancing policy: %s", s)
	}

	return policy, nil
}

func AllLBPolicies() []LBPolicy {
	return allLBPolicies
}

// =============================================================================
// HealthStatus
// =============================================================================

// HealthStatus represents the health status of a load balancer backend
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

var allHealthStatuses = []HealthStatus{HealthStatusHealthy, HealthStatusUnhealthy, HealthStatusUnknown}

var healthStatusLabels = map[HealthStatus]string{
	HealthStatusHealthy:   "Healthy",
	HealthStatusUnhealthy: "Unhealthy",
	HealthStatusUnknown:   "Unknown",
}

func (h HealthStatus) String() string {
	return string(h)
}

func (h HealthStatus) Label() string {
	return enumtypes.Label(h, healthStatusLabels, "Unknown")
}

func (h HealthStatus) IsValid() bool {
	return enumtypes.IsValid(h, allHealthStatuses...)
}

func (h *HealthStatus) Scan(value interface{}) error {
	if value == nil {
		*h = HealthStatusUnknown
		return nil
	}

	return enumtypes.ScanString(h, value)
}

func (h HealthStatus) Value() (driver.Value, error) {
	return enumtypes.ValueString(h)
}
