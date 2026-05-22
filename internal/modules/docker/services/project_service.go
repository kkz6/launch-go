package services

import (
	"context"
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

	out := make([]dto.ProjectResponse, 0, len(projects))
	for i := range projects {
		out = append(out, *dto.ToProjectResponse(&projects[i]))
	}
	return out, nil
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

// DeleteProject soft-deletes a project. Once we add workload services we'll
// extend this to reject deletion while workloads still exist (or cascade
// after a confirmation flow). For phase 1 we trust the soft-delete and the
// cascading FKs the migration set up.
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
