package services

import (
	"context"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ApplicationService handles business logic for docker applications.
type ApplicationService struct {
	*BaseService
}

// NewApplicationService wires the service from its dependency carrier.
func NewApplicationService(deps *ServiceDeps) *ApplicationService {
	return &ApplicationService{BaseService: NewBaseService(deps)}
}

// ListApplications returns every application inside a project, after
// verifying the project belongs to the (team, server) tuple. Returning
// 404 for a missing project (rather than 200 empty) means the frontend
// route can fail-fast on a bad project ID instead of silently rendering
// an empty list.
//
// Signature: fiberutil.IndexDoubleNestedFunc — (ctx, projectID,
// serverID, teamID) order.
func (s *ApplicationService) ListApplications(
	ctx context.Context, projectID, serverID, teamID string,
) ([]dto.ApplicationResponse, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	apps, err := s.Repos().Application().ListForProject(ctx, teamID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ApplicationResponse, 0, len(apps))
	for i := range apps {
		out = append(out, *dto.ToApplicationResponse(&apps[i]))
	}
	return out, nil
}

// GetApplication returns a single app by ID, after verifying it lives in
// the project the URL claims it does. Two scoping layers: the project
// lookup (404 if wrong team/server) AND the app's project_id (404 if the
// app exists but in a different project — a URL-tampering defence).
//
// Signature: fiberutil.ShowDoubleNestedFunc — (ctx, id, projectID,
// serverID, teamID).
func (s *ApplicationService) GetApplication(
	ctx context.Context, id, projectID, serverID, teamID string,
) (dto.ApplicationResponse, error) {
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if app.ProjectID != projectID {
		// App exists but belongs to a different project. Treat as NotFound
		// so an attacker can't enumerate apps by guessing URLs.
		return dto.ApplicationResponse{}, fiberutil.NotFound()
	}
	return *dto.ToApplicationResponse(app), nil
}

// CreateApplication registers a new docker application inside a project.
// Validates that:
//   - The project exists and belongs to the caller's team + server.
//   - The name is unique within the project.
//   - The source_type-specific payload is present and well-formed.
//
// The deploy job that actually pulls/builds/runs the container lands in
// slice 2b. For phase 2a, we just record the configuration with
// status=idle and broadcast docker.application.created.
//
// Signature: fiberutil.CreateDoubleNestedFunc — (ctx, projectID,
// serverID, teamID, userID, req).
func (s *ApplicationService) CreateApplication(
	ctx context.Context, projectID, serverID, teamID, userID string,
	req *dto.CreateApplicationRequest,
) (dto.ApplicationResponse, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.ApplicationResponse{}, fiberutil.BadRequest("Application name is required")
	}

	taken, err := s.Repos().Application().ExistsByNameInProject(ctx, projectID, name, "")
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if taken {
		return dto.ApplicationResponse{}, fiberutil.Conflict(
			"An application with that name already exists in this project",
		)
	}

	sourceConfig, buildType, buildConfig, err := buildSourceConfig(req)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}

	app := &models.Application{
		ProjectID:    projectID,
		Name:         name,
		SourceType:   dockertypes.SourceType(req.SourceType),
		SourceConfig: sourceConfig,
		BuildType:    buildType,
		BuildConfig:  buildConfig,
		Status:       dockertypes.ApplicationStatusIdle,
	}
	app.TeamID = teamID
	app.ServerID = serverID

	if err := s.Repos().Application().Create(ctx, app); err != nil {
		return dto.ApplicationResponse{}, err
	}

	resp := dto.ToApplicationResponse(app)
	s.BroadcastToTeam(teamID, "docker.application.created", resp)
	return *resp, nil
}

// UpdateApplication renames an application. Other fields are immutable in
// phase 2a (see UpdateApplicationRequest). If we widen this, the unique-
// name check needs to consider excludeID, which the repo already supports.
//
// Signature: fiberutil.UpdateDoubleNestedFunc.
func (s *ApplicationService) UpdateApplication(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
	req *dto.UpdateApplicationRequest,
) (dto.ApplicationResponse, error) {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return dto.ApplicationResponse{}, err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	if app.ProjectID != projectID {
		return dto.ApplicationResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return dto.ApplicationResponse{}, fiberutil.BadRequest("Application name cannot be empty")
		}
		if newName != app.Name {
			taken, err := s.Repos().Application().ExistsByNameInProject(ctx, projectID, newName, id)
			if err != nil {
				return dto.ApplicationResponse{}, err
			}
			if taken {
				return dto.ApplicationResponse{}, fiberutil.Conflict(
					"An application with that name already exists in this project",
				)
			}
			updates["name"] = newName
		}
	}

	if len(updates) > 0 {
		if err := s.Repos().Application().UpdateFields(ctx, id, updates); err != nil {
			return dto.ApplicationResponse{}, err
		}
	}

	reloaded, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ApplicationResponse{}, err
	}
	resp := dto.ToApplicationResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.application.updated", resp)
	return *resp, nil
}

// DeleteApplication soft-deletes an application. The actual `docker rm`
// of the running container lands in slice 2b alongside the deploy job —
// for phase 2a we trust the soft-delete and let the (future) reaper
// clean up the abandoned container.
//
// Signature: fiberutil.DeleteDoubleNestedFunc.
func (s *ApplicationService) DeleteApplication(
	ctx context.Context, id, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	if _, err := s.requireProject(ctx, projectID, serverID, teamID); err != nil {
		return err
	}
	app, err := s.Repos().Application().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}
	if app.ProjectID != projectID {
		return fiberutil.NotFound()
	}

	if err := s.Repos().Application().Delete(ctx, id); err != nil {
		return err
	}

	s.BroadcastToTeam(teamID, "docker.application.deleted", map[string]any{
		"id":         app.ID,
		"project_id": app.ProjectID,
		"server_id":  app.ServerID,
		"team_id":    app.TeamID,
	})
	return nil
}

// requireProject validates that the project exists and belongs to the
// (team, server). Returns the project ID on success — same shape as
// ProjectService.requireDockerServer so it slots into the same call
// pattern in each method.
func (s *ApplicationService) requireProject(
	ctx context.Context, projectID, serverID, teamID string,
) (string, error) {
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// buildSourceConfig translates the discriminated-union request payload
// into the (source_config jsonb, build_type, build_config jsonb) triple
// the model stores.
//
// Each branch verifies the matching nested struct is present so a client
// can't smuggle e.g. a Git payload while declaring source_type=image.
func buildSourceConfig(req *dto.CreateApplicationRequest) (
	source dbtype.JSONMap,
	buildType *dockertypes.BuildType,
	buildConfig dbtype.JSONMap,
	err error,
) {
	switch req.SourceType {
	case "image":
		if req.Image == nil || strings.TrimSpace(req.Image.Image) == "" {
			return nil, nil, nil, fiberutil.BadRequest("image source requires `image.image`")
		}
		source = dbtype.JSONMap{"image": strings.TrimSpace(req.Image.Image)}
		if req.Image.RegistryCredentialID != nil && *req.Image.RegistryCredentialID != "" {
			source["registry_credential_id"] = *req.Image.RegistryCredentialID
		}
		// No build step for pre-built images.

	case "git":
		if req.Git == nil ||
			strings.TrimSpace(req.Git.Repo) == "" ||
			strings.TrimSpace(req.Git.Branch) == "" {
			return nil, nil, nil, fiberutil.BadRequest("git source requires `git.repo` and `git.branch`")
		}
		source = dbtype.JSONMap{
			"repo":   strings.TrimSpace(req.Git.Repo),
			"branch": strings.TrimSpace(req.Git.Branch),
		}
		if req.Git.SourceControlID != nil && *req.Git.SourceControlID != "" {
			source["source_control_id"] = *req.Git.SourceControlID
		}
		// Build type defaults to nixpacks; the deploy job will auto-detect
		// a Dockerfile at the repo root and override unless the user has
		// explicitly chosen.
		bt := dockertypes.BuildTypeNixpacks
		if req.Git.BuildType != nil {
			bt = dockertypes.BuildType(*req.Git.BuildType)
		}
		buildType = &bt
		buildConfig = dbtype.JSONMap{}
		if req.Git.DockerfilePath != nil && strings.TrimSpace(*req.Git.DockerfilePath) != "" {
			buildConfig["dockerfile_path"] = strings.TrimSpace(*req.Git.DockerfilePath)
		}

	case "dockerfile":
		if req.Dockerfile == nil || strings.TrimSpace(req.Dockerfile.Contents) == "" {
			return nil, nil, nil, fiberutil.BadRequest("dockerfile source requires `dockerfile.contents`")
		}
		// We store the contents directly in source_config so a redeploy
		// uses the same Dockerfile without re-fetching. The 64 KiB cap on
		// the request struct keeps this bounded.
		source = dbtype.JSONMap{"contents": req.Dockerfile.Contents}
		bt := dockertypes.BuildTypeDockerfile
		buildType = &bt
		buildConfig = dbtype.JSONMap{}

	default:
		return nil, nil, nil, fiberutil.BadRequest("unsupported source_type")
	}
	return source, buildType, buildConfig, nil
}
