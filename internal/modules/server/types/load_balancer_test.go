package types

import (
	"testing"
)

func TestLBPolicy_String(t *testing.T) {
	tests := []struct {
		policy LBPolicy
		want   string
	}{
		{LBPolicyRoundRobin, "round_robin"},
		{LBPolicyLeastConn, "least_conn"},
		{LBPolicyIPHash, "ip_hash"},
		{LBPolicyFirst, "first"},
		{LBPolicyRandom, "random"},
	}

	for _, tt := range tests {
		if got := tt.policy.String(); got != tt.want {
			t.Errorf("LBPolicy(%q).String() = %q, want %q", tt.policy, got, tt.want)
		}
	}
}

func TestLBPolicy_Label(t *testing.T) {
	tests := []struct {
		policy LBPolicy
		want   string
	}{
		{LBPolicyRoundRobin, "Round Robin"},
		{LBPolicyLeastConn, "Least Connections"},
		{LBPolicyIPHash, "IP Hash (Sticky Sessions)"},
		{LBPolicyFirst, "First Available"},
		{LBPolicyRandom, "Random"},
		{LBPolicy("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.policy.Label(); got != tt.want {
			t.Errorf("LBPolicy(%q).Label() = %q, want %q", tt.policy, got, tt.want)
		}
	}
}

func TestLBPolicy_IsValid(t *testing.T) {
	valid := []LBPolicy{LBPolicyRoundRobin, LBPolicyLeastConn, LBPolicyIPHash, LBPolicyFirst, LBPolicyRandom}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("LBPolicy(%q).IsValid() = false, want true", p)
		}
	}

	invalid := []LBPolicy{"invalid", "", "weighted"}
	for _, p := range invalid {
		if p.IsValid() {
			t.Errorf("LBPolicy(%q).IsValid() = true, want false", p)
		}
	}
}

func TestParseLBPolicy(t *testing.T) {
	tests := []struct {
		input   string
		want    LBPolicy
		wantErr bool
	}{
		{"round_robin", LBPolicyRoundRobin, false},
		{"least_conn", LBPolicyLeastConn, false},
		{"ip_hash", LBPolicyIPHash, false},
		{"first", LBPolicyFirst, false},
		{"random", LBPolicyRandom, false},
		{"invalid", LBPolicyRoundRobin, true},
		{"", LBPolicyRoundRobin, true},
	}

	for _, tt := range tests {
		got, err := ParseLBPolicy(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLBPolicy(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLBPolicy(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAllLBPolicies(t *testing.T) {
	policies := AllLBPolicies()
	if len(policies) != 5 {
		t.Errorf("AllLBPolicies() returned %d items, want 5", len(policies))
	}

	for _, p := range policies {
		if !p.IsValid() {
			t.Errorf("AllLBPolicies() contains invalid policy: %q", p)
		}
	}
}

func TestLBPolicy_ScanValue(t *testing.T) {
	var p LBPolicy

	// Scan nil defaults to round_robin
	if err := p.Scan(nil); err != nil {
		t.Errorf("LBPolicy.Scan(nil) error = %v", err)
	}
	if p != LBPolicyRoundRobin {
		t.Errorf("LBPolicy.Scan(nil) = %q, want %q", p, LBPolicyRoundRobin)
	}

	// Scan string value
	if err := p.Scan("ip_hash"); err != nil {
		t.Errorf("LBPolicy.Scan(\"ip_hash\") error = %v", err)
	}
	if p != LBPolicyIPHash {
		t.Errorf("LBPolicy.Scan(\"ip_hash\") = %q, want %q", p, LBPolicyIPHash)
	}

	// Value round-trip
	val, err := LBPolicyRandom.Value()
	if err != nil {
		t.Errorf("LBPolicy.Value() error = %v", err)
	}
	if val != "random" {
		t.Errorf("LBPolicy.Value() = %v, want \"random\"", val)
	}
}

// --- HealthStatus tests ---

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   string
	}{
		{HealthStatusHealthy, "healthy"},
		{HealthStatusUnhealthy, "unhealthy"},
		{HealthStatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("HealthStatus(%q).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestHealthStatus_Label(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   string
	}{
		{HealthStatusHealthy, "Healthy"},
		{HealthStatusUnhealthy, "Unhealthy"},
		{HealthStatusUnknown, "Unknown"},
		{HealthStatus("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.Label(); got != tt.want {
			t.Errorf("HealthStatus(%q).Label() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestHealthStatus_IsValid(t *testing.T) {
	valid := []HealthStatus{HealthStatusHealthy, HealthStatusUnhealthy, HealthStatusUnknown}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("HealthStatus(%q).IsValid() = false, want true", s)
		}
	}

	if HealthStatus("invalid").IsValid() {
		t.Error("HealthStatus(\"invalid\").IsValid() = true, want false")
	}
}

func TestHealthStatus_ScanValue(t *testing.T) {
	var h HealthStatus

	// Scan nil defaults to unknown
	if err := h.Scan(nil); err != nil {
		t.Errorf("HealthStatus.Scan(nil) error = %v", err)
	}
	if h != HealthStatusUnknown {
		t.Errorf("HealthStatus.Scan(nil) = %q, want %q", h, HealthStatusUnknown)
	}

	// Scan string value
	if err := h.Scan("healthy"); err != nil {
		t.Errorf("HealthStatus.Scan(\"healthy\") error = %v", err)
	}
	if h != HealthStatusHealthy {
		t.Errorf("HealthStatus.Scan(\"healthy\") = %q, want %q", h, HealthStatusHealthy)
	}

	// Value round-trip
	val, err := HealthStatusUnhealthy.Value()
	if err != nil {
		t.Errorf("HealthStatus.Value() error = %v", err)
	}
	if val != "unhealthy" {
		t.Errorf("HealthStatus.Value() = %v, want \"unhealthy\"", val)
	}
}
