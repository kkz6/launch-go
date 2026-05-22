package dto

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
)

// EnvVar secret-masking is the most fragile bit of the response layer:
// a regression here would leak passwords into list endpoints. Pin it.
func TestToEnvVarResponse_MasksSecretsByDefault(t *testing.T) {
	v := &models.ApplicationEnvVar{
		Key:      "DATABASE_PASSWORD",
		Value:    "super-secret",
		IsSecret: true,
	}
	got := ToEnvVarResponse(v, false)
	if got.Value == "super-secret" {
		t.Fatalf("default response leaked secret value")
	}
	if got.Value != "********" {
		t.Errorf("expected mask, got %q", got.Value)
	}
}

func TestToEnvVarResponse_RevealsWhenAsked(t *testing.T) {
	v := &models.ApplicationEnvVar{Key: "K", Value: "secret", IsSecret: true}
	got := ToEnvVarResponse(v, true)
	if got.Value != "secret" {
		t.Errorf("reveal=true should return cleartext, got %q", got.Value)
	}
}

func TestToEnvVarResponse_NonSecretAlwaysCleartext(t *testing.T) {
	// IsSecret=false vars must never be masked — they're public config
	// and seeing them mask in the UI would suggest a stored secret
	// where none exists.
	v := &models.ApplicationEnvVar{Key: "NODE_ENV", Value: "production", IsSecret: false}
	if got := ToEnvVarResponse(v, false).Value; got != "production" {
		t.Errorf("non-secret value should be returned as-is, got %q", got)
	}
}

func TestToApplicationResponse_IncludesInternalPort(t *testing.T) {
	// Regression check — internal_port lives on a follow-up migration
	// and we've already had a frontend vue-tsc fail because the type
	// was missing on the response.
	a := &models.Application{Name: "api", InternalPort: 3000}
	got := ToApplicationResponse(a)
	if got.InternalPort != 3000 {
		t.Errorf("expected internal_port 3000, got %d", got.InternalPort)
	}
}

func TestToComposeResponse_OmitsRawYAMLOnListResponses(t *testing.T) {
	// list endpoints should never ship the raw YAML — it can be KB
	// scale and there's no reason to inflate every list response.
	raw := "version: '3'"
	c := &models.Compose{Name: "stack", RawYAML: &raw}
	got := ToComposeResponse(c, false)
	if got.RawYAML != nil {
		t.Errorf("list response should omit raw_yaml, got %q", *got.RawYAML)
	}
	got = ToComposeResponse(c, true)
	if got.RawYAML == nil || *got.RawYAML != raw {
		t.Errorf("show response should include raw_yaml; got %v", got.RawYAML)
	}
}
