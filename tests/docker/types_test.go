package docker_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/types"
)

func TestServiceKind_IsValid(t *testing.T) {
	tests := []struct {
		kind  types.ServiceKind
		valid bool
	}{
		{types.ServiceKindApplication, true},
		{types.ServiceKindService, true},
		{types.ServiceKind("invalid"), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			if tc.kind.IsValid() != tc.valid {
				t.Errorf("expected IsValid()=%v for %q", tc.valid, tc.kind)
			}
		})
	}
}

func TestServiceStatus_IsValid(t *testing.T) {
	validStatuses := []types.ServiceStatus{
		types.ServiceStatusPending,
		types.ServiceStatusDeploying,
		types.ServiceStatusRunning,
		types.ServiceStatusStopped,
		types.ServiceStatusFailed,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
		if s.Label() == "Unknown" {
			t.Errorf("expected label for %q, got 'Unknown'", s)
		}
	}

	if types.ServiceStatus("bogus").IsValid() {
		t.Error("expected 'bogus' to be invalid")
	}
}

func TestRestartPolicy_IsValid(t *testing.T) {
	validPolicies := []types.RestartPolicy{
		types.RestartPolicyNo,
		types.RestartPolicyAlways,
		types.RestartPolicyUnlessStopped,
		types.RestartPolicyOnFailure,
	}

	for _, p := range validPolicies {
		if !p.IsValid() {
			t.Errorf("expected %q to be valid", p)
		}
	}

	if types.RestartPolicy("bogus").IsValid() {
		t.Error("expected 'bogus' to be invalid")
	}
}

func TestDeploymentStatus_IsValid(t *testing.T) {
	validStatuses := []types.DeploymentStatus{
		types.DeploymentStatusPending,
		types.DeploymentStatusRunning,
		types.DeploymentStatusFinished,
		types.DeploymentStatusFailed,
	}

	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
}

func TestDeploymentTrigger_IsValid(t *testing.T) {
	validTriggers := []types.DeploymentTrigger{
		types.DeploymentTriggerManual,
		types.DeploymentTriggerWebhook,
		types.DeploymentTriggerComposeUpdate,
	}

	for _, tr := range validTriggers {
		if !tr.IsValid() {
			t.Errorf("expected %q to be valid", tr)
		}
	}
}

func TestMountType_IsValid(t *testing.T) {
	if !types.MountTypeVolume.IsValid() {
		t.Error("expected MountTypeVolume to be valid")
	}

	if !types.MountTypeBind.IsValid() {
		t.Error("expected MountTypeBind to be valid")
	}

	if types.MountType("tmpfs").IsValid() {
		t.Error("expected 'tmpfs' to be invalid")
	}
}

func TestCertificateType_IsValid(t *testing.T) {
	validTypes := []types.CertificateType{
		types.CertificateTypeLetsEncrypt,
		types.CertificateTypeCustom,
		types.CertificateTypeNone,
	}

	for _, ct := range validTypes {
		if !ct.IsValid() {
			t.Errorf("expected %q to be valid", ct)
		}
	}
}

func TestProtocol_IsValid(t *testing.T) {
	if !types.ProtocolTCP.IsValid() {
		t.Error("expected TCP to be valid")
	}

	if !types.ProtocolUDP.IsValid() {
		t.Error("expected UDP to be valid")
	}

	if types.Protocol("sctp").IsValid() {
		t.Error("expected 'sctp' to be invalid")
	}
}

func TestServiceKind_Label(t *testing.T) {
	if types.ServiceKindApplication.Label() != "Application" {
		t.Errorf("expected 'Application', got %q", types.ServiceKindApplication.Label())
	}

	if types.ServiceKindService.Label() != "Service" {
		t.Errorf("expected 'Service', got %q", types.ServiceKindService.Label())
	}
}

func TestServiceStatus_Label(t *testing.T) {
	tests := map[types.ServiceStatus]string{
		types.ServiceStatusPending:   "Pending",
		types.ServiceStatusDeploying: "Deploying",
		types.ServiceStatusRunning:   "Running",
		types.ServiceStatusStopped:   "Stopped",
		types.ServiceStatusFailed:    "Failed",
	}

	for status, expectedLabel := range tests {
		if status.Label() != expectedLabel {
			t.Errorf("expected label %q for %q, got %q", expectedLabel, status, status.Label())
		}
	}
}
