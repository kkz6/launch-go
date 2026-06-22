package dto

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
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

// A GitHub Actions app whose build secrets changed since the last
// workflow sync must report out-of-sync so the UI shows the
// "re-sync workflow" banner with a pending count. The counter comes
// back from JSON as a float64.
func TestToApplicationResponse_GHAOutOfSyncWhenPendingChanges(t *testing.T) {
	a := &models.Application{
		BuildLocation: dockertypes.BuildLocationGitHubActions,
		SourceConfig:  dbtype.JSONMap{"gha_pending_changes": float64(2)},
	}
	got := ToApplicationResponse(a)
	if !got.GHAOutOfSync {
		t.Errorf("github_actions app with pending changes must report out of sync")
	}
	if got.GHAPendingChanges != 2 {
		t.Errorf("expected pending_changes 2, got %d", got.GHAPendingChanges)
	}
}

// No pending changes → in sync, banner hidden.
func TestToApplicationResponse_GHAInSyncWhenNoPending(t *testing.T) {
	a := &models.Application{
		BuildLocation: dockertypes.BuildLocationGitHubActions,
		SourceConfig:  dbtype.JSONMap{},
	}
	got := ToApplicationResponse(a)
	if got.GHAOutOfSync {
		t.Errorf("github_actions app with no pending changes must be in sync")
	}
	if got.GHAPendingChanges != 0 {
		t.Errorf("expected 0 pending, got %d", got.GHAPendingChanges)
	}
}

// Server-build apps never have a workflow to sync — out-of-sync is
// always false and the pending count is suppressed, even if stale
// source_config carries a counter.
func TestToApplicationResponse_ServerBuildNeverOutOfSync(t *testing.T) {
	a := &models.Application{
		BuildLocation: dockertypes.BuildLocationServer,
		SourceConfig:  dbtype.JSONMap{"gha_pending_changes": float64(5)},
	}
	got := ToApplicationResponse(a)
	if got.GHAOutOfSync {
		t.Errorf("server-build app must never report out of sync")
	}
	if got.GHAPendingChanges != 0 {
		t.Errorf("server-build pending must be 0, got %d", got.GHAPendingChanges)
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
