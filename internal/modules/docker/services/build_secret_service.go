package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// BuildSecretService manages build-time secrets on a docker application.
// These values are NEVER passed to `docker run` — they're mounted into
// `docker build` via BuildKit's --mount=type=secret. Same auth/scoping
// rules as the env-var service.
//
// Mutations are eventually propagated to the build environment:
//
//   - server-build path: read at deploy time from this table and
//     materialised to a 0600 tmpfs file under /run/launch.
//   - GHA path: each save also enqueues a workflow re-sync so the
//     workflow YAML's `secrets:` block lists every current name and
//     the corresponding repo secret (LAUNCH_BUILD_<NAME>) is pushed.
//
// The re-sync hook is fire-and-forget — the API call returns
// successfully whether or not the GHA push succeeds; the user sees
// the failure via the same WS events the bootstrap job uses.
type BuildSecretService struct {
	*BaseService
}

func NewBuildSecretService(deps *ServiceDeps) *BuildSecretService {
	return &BuildSecretService{BaseService: NewBaseService(deps)}
}

// ListBuildSecrets returns build secrets for an application. Values
// are never included — see BuildSecretResponse for the API shape.
func (s *BuildSecretService) ListBuildSecrets(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.BuildSecretResponse, error) {
	if _, err := s.scopedAppForBuildSecret(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().BuildSecret().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return mapResponseValues(rows, dto.ToApplicationBuildSecretResponse), nil
}

// CreateBuildSecret adds a name/value to the application. (app, name)
// is unique per live row; we return 409 before the DB index throws so
// the message is friendly.
func (s *BuildSecretService) CreateBuildSecret(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateBuildSecretRequest,
) (dto.BuildSecretResponse, error) {
	_ = userID
	app, err := s.scopedAppForBuildSecret(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if !validBuildSecretName.MatchString(name) {
		return dto.BuildSecretResponse{}, fiberutil.BadRequest(
			"Build secret name must start with a letter or underscore and contain only letters, digits, or underscores",
		)
	}
	taken, err := s.Repos().BuildSecret().ExistsByName(ctx, applicationID, name, "")
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	if taken {
		return dto.BuildSecretResponse{}, fiberutil.Conflict(
			"A build secret with that name already exists on this application",
		)
	}

	v := &models.ApplicationBuildSecret{
		ApplicationID: applicationID,
		Name:          name,
		Value:         dbtype.EncryptedString(req.Value),
	}
	if err := s.Repos().BuildSecret().Create(ctx, v); err != nil {
		return dto.BuildSecretResponse{}, err
	}

	resp := dto.ToApplicationBuildSecretResponse(v)
	s.BroadcastToTeam(teamID, "docker.application.build_secret.added", map[string]any{
		"application_id":  app.ID,
		"server_id":       app.ServerID,
		"team_id":         app.TeamID,
		"build_secret_id": v.ID,
		"name":            v.Name,
	})
	s.markGHADirty(ctx, app, teamID)
	return *resp, nil
}

// UpdateBuildSecret replaces the value. Name is immutable — Dockerfile
// references would silently break otherwise. Delete + Create to rename.
func (s *BuildSecretService) UpdateBuildSecret(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateBuildSecretRequest,
) (dto.BuildSecretResponse, error) {
	_ = userID
	app, err := s.scopedAppForBuildSecret(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	v, err := s.Repos().BuildSecret().FindByID(ctx, id)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	if v.ApplicationID != applicationID {
		return dto.BuildSecretResponse{}, fiberutil.NotFound()
	}

	// Always-write update: the request mandates `value`, so there's no
	// partial path here. Wrap as EncryptedString so the type's Valuer
	// re-encrypts before write.
	if err := s.Repos().BuildSecret().UpdateFields(ctx, id, map[string]any{
		"value": dbtype.EncryptedString(req.Value),
	}); err != nil {
		return dto.BuildSecretResponse{}, err
	}
	reloaded, err := s.Repos().BuildSecret().FindByID(ctx, id)
	if err != nil {
		return dto.BuildSecretResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.build_secret.updated", map[string]any{
		"application_id":  app.ID,
		"server_id":       app.ServerID,
		"team_id":         app.TeamID,
		"build_secret_id": id,
		"name":            reloaded.Name,
	})
	s.markGHADirty(ctx, app, teamID)
	return *dto.ToApplicationBuildSecretResponse(reloaded), nil
}

// DeleteBuildSecret soft-deletes the row + triggers a workflow resync
// when the app is GHA-backed (so the YAML stops referencing a name
// that no longer exists; the orphan repo secret on GitHub is left in
// place — harmless and out of scope here).
func (s *BuildSecretService) DeleteBuildSecret(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedAppForBuildSecret(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	v, err := s.Repos().BuildSecret().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	name := v.Name
	if err := s.Repos().BuildSecret().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.application.build_secret.deleted", map[string]any{
		"application_id":  app.ID,
		"server_id":       app.ServerID,
		"team_id":         app.TeamID,
		"build_secret_id": id,
		"name":            name,
	})
	s.markGHADirty(ctx, app, teamID)
	return nil
}

// scopedAppForBuildSecret validates the project + application chain
// and returns the application so the caller can broadcast routing
// fields and check build_location.
func (s *BuildSecretService) scopedAppForBuildSecret(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) (*models.Application, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, applicationID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if app.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return app, nil
}

// markGHADirty records that build secrets changed so the committed
// workflow YAML + repo secrets are now stale — WITHOUT pushing
// anything to GitHub. It bumps the pending-changes counter on
// source_config and broadcasts gha_out_of_sync so the UI shows a
// "re-sync workflow" banner; the user applies the changes explicitly
// via the resync action. No-op for server-build apps — they read the
// build-secret table directly at deploy time.
//
// Best-effort: a failure to persist the counter is logged, not
// returned. The API caller already saw a 2xx for the secret mutation;
// bouncing here would give a confusing "secret saved but error" state.
func (s *BuildSecretService) markGHADirty(ctx context.Context, app *models.Application, teamID string) {
	if app == nil || app.BuildLocation != dockertypes.BuildLocationGitHubActions {
		return
	}
	cfg, pending := bumpGHAPendingChanges(app.SourceConfig)
	app.SourceConfig = cfg
	if err := s.Repos().Application().UpdateSourceConfig(ctx, app.ID, cfg); err != nil {
		if s.Logger != nil {
			s.Logger.Warn().Err(err).Str("application_id", app.ID).
				Msg("build-secret: failed to persist gha pending-changes counter")
		}
		return
	}
	s.BroadcastToTeam(teamID, "docker.application.gha_out_of_sync", map[string]any{
		"application_id":  app.ID,
		"server_id":       app.ServerID,
		"team_id":         app.TeamID,
		"pending_changes": pending,
	})
}

// validBuildSecretName mirrors validEnvVarKey: POSIX env-name shape
// (letters/digits/underscores, not starting with a digit). Same
// rationale — these are referenced by id=NAME from Dockerfiles which
// use shell variable conventions.
var validBuildSecretName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
