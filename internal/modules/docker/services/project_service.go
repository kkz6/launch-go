package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// ProjectService handles docker-project business logic.
type ProjectService struct {
	*BaseService
}

// NewProjectService wires the service from its dependency carrier.
func NewProjectService(deps *ServiceDeps) *ProjectService {
	return &ProjectService{BaseService: NewBaseService(deps)}
}

// ListProjects returns every project on a server, scoped to the caller's
// team. Returns an empty slice (never nil) so the JSON response is a
// stable `[]`.
//
// Signature matches fiberutil.IndexNestedFunc.
func (s *ProjectService) ListProjects(
	ctx context.Context, serverID, teamID string,
) ([]dto.ProjectResponse, error) {
	if _, err := s.requireDockerServer(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	projects, err := s.Repos().Project().ListForServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}

	return mapResponseValues(projects, dto.ToProjectResponse), nil
}

// GetProject returns a single project by ID, with workload counts populated.
//
// Signature matches fiberutil.ShowNestedFunc.
func (s *ProjectService) GetProject(
	ctx context.Context, id, serverID, teamID string,
) (dto.ProjectResponse, error) {
	if _, err := s.requireDockerServer(ctx, serverID, teamID); err != nil {
		return dto.ProjectResponse{}, err
	}
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	return *dto.ToProjectResponse(p), nil
}

// CreateProject creates a new project on the server after validating
// that the server is a docker server and the name is unique.
//
// Signature matches fiberutil.CreateNestedFunc.
func (s *ProjectService) CreateProject(
	ctx context.Context, serverID, teamID, userID string, req *dto.CreateProjectRequest,
) (dto.ProjectResponse, error) {
	_ = userID
	if _, err := s.requireDockerServer(ctx, serverID, teamID); err != nil {
		return dto.ProjectResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.ProjectResponse{}, fiberutil.BadRequest("Project name is required")
	}

	taken, err := s.Repos().Project().ExistsByName(ctx, serverID, name, "")
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	if taken {
		return dto.ProjectResponse{}, fiberutil.Conflict("A project with that name already exists on this server")
	}

	p := &models.Project{
		Name:        name,
		Description: req.Description,
	}
	p.TeamID = teamID
	p.ServerID = serverID

	if err := s.Repos().Project().Create(ctx, p); err != nil {
		return dto.ProjectResponse{}, err
	}

	resp := dto.ToProjectResponse(p)
	s.BroadcastToTeam(teamID, "docker.project.created", resp)
	return *resp, nil
}

// UpdateProject applies a partial update. Renaming hits the same uniqueness
// check as create so two projects can't end up with the same name.
//
// Signature matches fiberutil.UpdateNestedFunc.
func (s *ProjectService) UpdateProject(
	ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateProjectRequest,
) (dto.ProjectResponse, error) {
	_ = userID
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ProjectResponse{}, err
	}

	updates := map[string]any{}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return dto.ProjectResponse{}, fiberutil.BadRequest("Project name cannot be empty")
		}
		if newName != p.Name {
			taken, err := s.Repos().Project().ExistsByName(ctx, serverID, newName, id)
			if err != nil {
				return dto.ProjectResponse{}, err
			}
			if taken {
				return dto.ProjectResponse{}, fiberutil.Conflict("A project with that name already exists on this server")
			}
			updates["name"] = newName
		}
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if len(updates) > 0 {
		if err := s.Repos().Project().UpdateFields(ctx, id, updates); err != nil {
			return dto.ProjectResponse{}, err
		}
	}

	reloaded, err := s.Repos().Project().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	resp := dto.ToProjectResponse(reloaded)
	s.BroadcastToTeam(teamID, "docker.project.updated", resp)
	return *resp, nil
}

// DeleteProject soft-deletes a project. Hard-blocks if any live
// applications / composes / databases still belong to it — the user
// must remove every workload first. Matches how sites refuse to delete
// while active deployments are tied to them.
//
// We could cascade via FK + dispatch docker-rm jobs for every
// workload, but a project with workloads is almost always "I clicked
// the wrong button" rather than "I really want to wipe this group of
// infra". A hard refusal is a safer default — the user can still
// delete each workload individually from its own detail page.
//
// Signature matches fiberutil.DeleteNestedFunc.
func (s *ProjectService) DeleteProject(
	ctx context.Context, id, serverID, teamID, userID string,
) error {
	_ = userID
	p, err := s.Repos().Project().FindByIDAndTeamServer(ctx, id, teamID, serverID)
	if err != nil {
		return err
	}

	apps, composes, databases, err := s.Repos().Project().CountWorkloads(ctx, id)
	if err != nil {
		return err
	}
	total := apps + composes + databases
	if total > 0 {
		// Build a per-kind summary so the toast tells the user
		// exactly what's left. UI also reads counts off the project
		// model and disables the Delete button proactively — this
		// error is the backstop.
		parts := []string{}
		if apps > 0 {
			parts = append(parts, fmt.Sprintf("%d application(s)", apps))
		}
		if composes > 0 {
			parts = append(parts, fmt.Sprintf("%d compose stack(s)", composes))
		}
		if databases > 0 {
			parts = append(parts, fmt.Sprintf("%d database(s)", databases))
		}
		// fiberutil.Validation maps to HTTP 422, which the frontend
		// surfaces as a toast with the message text intact.
		return fiberutil.Validation(fmt.Sprintf(
			"This project still has %s. Remove each one from its detail page first, then delete the project.",
			strings.Join(parts, ", "),
		))
	}

	if err := s.Repos().Project().Delete(ctx, id); err != nil {
		return err
	}

	s.BroadcastToTeam(teamID, "docker.project.deleted", map[string]any{
		"id":        p.ID,
		"server_id": p.ServerID,
		"team_id":   p.TeamID,
	})
	return nil
}

// requireDockerServer ensures the server exists, belongs to the team, and
// is actually a docker server. Other server types don't get docker
// projects (a "Database" server has no docker engine installed).
func (s *ProjectService) requireDockerServer(
	ctx context.Context, serverID, teamID string,
) (string, error) {
	server, err := s.ServerRepos().Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return "", err
	}
	if server.Type == nil || *server.Type != string(servertypes.ServerTypeDocker) {
		return "", fiberutil.BadRequest("Server is not a docker server")
	}
	return server.ID, nil
}
