package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
)

// These tests exercise the GHA-deploy dispatch path WITHOUT a full
// service rig — by hitting the package-level dispatchGHADeploy helper
// and the parser directly. They reproduce the production bug the user
// hit ("Deploy on a GHA app silently runs the on-server build") and
// pin the fix in place.

// fakeDispatcher records the arguments TriggerWorkflowDispatch was
// called with so tests can assert the right values flow through.
// Returns whatever Err is set to so tests can simulate GitHub
// failures (404 workflow missing, 403 permission denied, etc.).
type fakeDispatcher struct {
	Calls []dispatchCall
	Err   error
}

type dispatchCall struct {
	InstallationID, Owner, Repo, WorkflowFile, Branch string
}

func (f *fakeDispatcher) TriggerWorkflowDispatch(
	ctx context.Context, installationID, owner, repo, workflowFile, branch string,
) error {
	f.Calls = append(f.Calls, dispatchCall{installationID, owner, repo, workflowFile, branch})
	return f.Err
}

// dispatchTestDB stands up an in-memory sqlite DB with just the
// source_controls table populated — enough for
// resolveGitHubInstallationID to find a row. We auto-migrate the git
// SourceControl model so we don't have to keep DDL in sync by hand.
//
// Reuses the package's strPtr helper (defined in database_service.go).
func dispatchTestDB(t *testing.T, installationID string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&gitmodels.SourceControl{}); err != nil {
		t.Fatalf("auto-migrate source_control: %v", err)
	}
	name := "test"
	inst := installationID
	sc := &gitmodels.SourceControl{
		Provider:       "github",
		ProviderID:     "test-provider-id",
		Name:           &name,
		InstallationID: &inst,
	}
	sc.ID = "sc_test"
	sc.TeamID = "team_test"
	if err := db.Create(sc).Error; err != nil {
		t.Fatalf("insert source_control: %v", err)
	}
	return db
}

func TestDispatchGHADeploy_HappyPath_PassesParsedFieldsToProvider(t *testing.T) {
	db := dispatchTestDB(t, "inst-42")
	gh := &fakeDispatcher{}

	sourceConfig := map[string]any{
		"source_control_id": "sc_test",
		"repo":              "git@github.com:GlobalJapan/gj-hrms-api.git",
		"branch":            "main",
		"gha_workflow_sha":  "abc1234", // bootstrap completed
		"gha_workflow_path": ".github/workflows/launch-deploy.yml",
	}

	if err := dispatchGHADeploy(context.Background(), db, gh, sourceConfig); err != nil {
		t.Fatalf("dispatchGHADeploy returned error: %v", err)
	}
	if len(gh.Calls) != 1 {
		t.Fatalf("expected 1 dispatch call, got %d", len(gh.Calls))
	}
	got := gh.Calls[0]
	want := dispatchCall{
		InstallationID: "inst-42",
		Owner:          "GlobalJapan",
		Repo:           "gj-hrms-api",
		WorkflowFile:   "launch-deploy.yml",
		Branch:         "main",
	}
	if got != want {
		t.Errorf("dispatch args mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestDispatchGHADeploy_RejectsWhenBootstrapNotFinished(t *testing.T) {
	db := dispatchTestDB(t, "inst-42")
	gh := &fakeDispatcher{}

	// Missing gha_workflow_sha → bootstrap hasn't committed the
	// workflow file yet → dispatch would 404. Surface a clean 409
	// instead of letting it fail at GitHub.
	sourceConfig := map[string]any{
		"source_control_id": "sc_test",
		"repo":              "git@github.com:GlobalJapan/gj-hrms-api.git",
		"branch":            "main",
		// no gha_workflow_sha
	}

	err := dispatchGHADeploy(context.Background(), db, gh, sourceConfig)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if len(gh.Calls) != 0 {
		t.Errorf("expected NO dispatch call when bootstrap is pending, got %d", len(gh.Calls))
	}
	// Caller maps this to fiberutil.Conflict — assert the message
	// shape so the UI guidance stays in sync.
	if got := err.Error(); !strings.Contains(got, "GitHub Actions setup hasn't finished") {
		t.Errorf("error message should explain bootstrap state, got %q", got)
	}
}

func TestDispatchGHADeploy_MapsWorkflowNotFoundToConflict(t *testing.T) {
	db := dispatchTestDB(t, "inst-42")
	gh := &fakeDispatcher{Err: gitproviders.ErrWorkflowNotFound}

	sourceConfig := map[string]any{
		"source_control_id": "sc_test",
		"repo":              "git@github.com:Owner/Repo.git",
		"branch":            "main",
		"gha_workflow_sha":  "abc1234",
	}

	err := dispatchGHADeploy(context.Background(), db, gh, sourceConfig)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "Re-sync workflow") {
		t.Errorf("error message should point to Re-sync, got %q", got)
	}
}

func TestDispatchGHADeploy_MapsInstallationGoneToValidation(t *testing.T) {
	db := dispatchTestDB(t, "inst-42")
	gh := &fakeDispatcher{Err: gitproviders.ErrInstallationNotFound}

	sourceConfig := map[string]any{
		"source_control_id": "sc_test",
		"repo":              "git@github.com:Owner/Repo.git",
		"branch":            "main",
		"gha_workflow_sha":  "abc1234",
	}

	err := dispatchGHADeploy(context.Background(), db, gh, sourceConfig)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "Reinstall the GitHub App") {
		t.Errorf("error message should point user at reconnecting the App, got %q", got)
	}
}

func TestDispatchGHADeploy_BubblesUnknownError(t *testing.T) {
	db := dispatchTestDB(t, "inst-42")
	sentinel := errors.New("network unreachable")
	gh := &fakeDispatcher{Err: sentinel}

	sourceConfig := map[string]any{
		"source_control_id": "sc_test",
		"repo":              "git@github.com:Owner/Repo.git",
		"branch":            "main",
		"gha_workflow_sha":  "abc1234",
	}

	err := dispatchGHADeploy(context.Background(), db, gh, sourceConfig)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error to bubble, got %v", err)
	}
}

func TestParseGHADispatchFields_AcceptsSSHCloneURL(t *testing.T) {
	got, err := parseGHADispatchFields(map[string]any{
		"source_control_id": "sc",
		"repo":              "git@github.com:GlobalJapan/gj-hrms-api.git",
		"gha_workflow_sha":  "sha",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Owner != "GlobalJapan" || got.Repo != "gj-hrms-api" {
		t.Errorf("owner/repo parse mismatch: %+v", got)
	}
	if got.Branch != "main" {
		t.Errorf("branch should default to 'main', got %q", got.Branch)
	}
	if got.WorkflowFile != "launch-deploy.yml" {
		t.Errorf("workflow file should default basename, got %q", got.WorkflowFile)
	}
}

func TestParseGHADispatchFields_AcceptsHTTPSCloneURL(t *testing.T) {
	got, err := parseGHADispatchFields(map[string]any{
		"source_control_id": "sc",
		"repo":              "https://github.com/GlobalJapan/gj-hrms-api",
		"gha_workflow_sha":  "sha",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Owner != "GlobalJapan" || got.Repo != "gj-hrms-api" {
		t.Errorf("https URL parse mismatch: %+v", got)
	}
}

func TestParseGHADispatchFields_AcceptsExplicitOwnerRepo(t *testing.T) {
	got, err := parseGHADispatchFields(map[string]any{
		"source_control_id": "sc",
		"owner":             "GlobalJapan",
		"repo":              "gj-hrms-api",
		"branch":            "develop",
		"gha_workflow_sha":  "sha",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Owner != "GlobalJapan" || got.Repo != "gj-hrms-api" {
		t.Errorf("explicit owner/repo parse mismatch: %+v", got)
	}
	if got.Branch != "develop" {
		t.Errorf("branch should be 'develop', got %q", got.Branch)
	}
}

func TestParseGHADispatchFields_MissingSourceControlIDFails(t *testing.T) {
	_, err := parseGHADispatchFields(map[string]any{
		"repo":             "owner/repo",
		"gha_workflow_sha": "sha",
	})
	if err == nil || !strings.Contains(err.Error(), "source_control_id") {
		t.Errorf("expected source_control_id error, got %v", err)
	}
}

func TestParseGHADispatchFields_RespectsCustomWorkflowPath(t *testing.T) {
	got, err := parseGHADispatchFields(map[string]any{
		"source_control_id": "sc",
		"repo":              "git@github.com:Owner/Repo.git",
		"gha_workflow_sha":  "sha",
		"gha_workflow_path": ".github/workflows/custom.yml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WorkflowFile != "custom.yml" {
		t.Errorf("workflow basename mismatch, got %q", got.WorkflowFile)
	}
}
