package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// gha_polish.go gathers the small endpoints the GitHub Actions detail
// subtab calls: rotate token, re-sync workflow, disable cleanup.
//
// Rotate + Re-sync are thin wrappers around gha:bootstrap_workflow
// with different RotateToken flags. The bootstrap job does the actual
// GitHub-side work and broadcasts the synced event when done.
//
// Disable currently does only the LOCAL cleanup — flips
// build_location back to "server", clears the token hash, drops the
// gha_* keys from source_config. The GitHub-side cleanup (delete
// secret + variables, comment-orphan the workflow file) is a
// follow-up tracked in the design doc's open-items list. Until then,
// the workflow file in the customer's repo keeps running — but
// without a valid LAUNCH_DEPLOY_TOKEN secret, the notify step in the
// workflow will 401 and the deploy won't fire. Acceptable failure
// mode for a v1 disable.

// --- ApplicationService methods --------------------------------------

func (s *ApplicationService) RotateGHAToken(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
) (dto.GHATokenRotateResponse, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	if app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return dto.GHATokenRotateResponse{}, fiberutil.Validation("Application is not configured for GitHub Actions builds")
	}
	task, err := jobs.NewGHABootstrapWorkflowTask("application", app.ID, true, s.AppURL())
	if err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	return dto.GHATokenRotateResponse{
		Status:  "queued",
		Message: "Token rotation queued. The new value will be live in GitHub Actions shortly.",
	}, nil
}

func (s *ApplicationService) ResyncGHA(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return err
	}
	if app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fiberutil.Validation("Application is not configured for GitHub Actions builds")
	}
	task, err := jobs.NewGHABootstrapWorkflowTask("application", app.ID, false, s.AppURL())
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

func (s *ApplicationService) DisableGHA(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return err
	}
	if app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fiberutil.Validation("Application is not configured for GitHub Actions builds")
	}
	if err := s.Repos().Application().UpdateFields(ctx, app.ID, map[string]any{
		"build_location":        dockertypes.BuildLocationServer,
		"gha_deploy_token_hash": nil,
		"source_config":         stripGHAFields(app.SourceConfig),
	}); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.application.gha_disabled", map[string]any{
		"application_id": app.ID,
	})
	return nil
}

// --- ComposeService methods ------------------------------------------

func (s *ComposeService) RotateGHAToken(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
) (dto.GHATokenRotateResponse, error) {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	compose, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	if compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return dto.GHATokenRotateResponse{}, fiberutil.Validation("Compose stack is not configured for GitHub Actions builds")
	}
	task, err := jobs.NewGHABootstrapWorkflowTask("compose", compose.ID, true, s.AppURL())
	if err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	if err := s.EnqueueTask(task); err != nil {
		return dto.GHATokenRotateResponse{}, err
	}
	return dto.GHATokenRotateResponse{
		Status:  "queued",
		Message: "Token rotation queued. The new value will be live in GitHub Actions shortly.",
	}, nil
}

func (s *ComposeService) ResyncGHA(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	compose, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return err
	}
	if compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fiberutil.Validation("Compose stack is not configured for GitHub Actions builds")
	}
	task, err := jobs.NewGHABootstrapWorkflowTask("compose", compose.ID, false, s.AppURL())
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

func (s *ComposeService) DisableGHA(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProjectScoped(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	compose, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return err
	}
	if compose.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return fiberutil.Validation("Compose stack is not configured for GitHub Actions builds")
	}
	if err := s.Repos().Compose().UpdateFields(ctx, compose.ID, map[string]any{
		"build_location":        dockertypes.BuildLocationServer,
		"gha_deploy_token_hash": nil,
		"source_config":         stripGHAFields(compose.SourceConfig),
	}); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.compose.gha_disabled", map[string]any{
		"compose_id": compose.ID,
	})
	return nil
}

// stripGHAFields returns a NEW source_config map with gha_* keys
// stripped. Keeps repo / branch / source_control_id so a future
// re-enable can pick up where the user left off.
func stripGHAFields(orig map[string]any) map[string]any {
	out := make(map[string]any, len(orig))
	for k, v := range orig {
		if strings.HasPrefix(k, "gha_") {
			continue
		}
		out[k] = v
	}
	return out
}
