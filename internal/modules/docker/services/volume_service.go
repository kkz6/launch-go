package services

import (
	"context"
	"regexp"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/docker/dto"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// VolumeService manages volumes attached to a docker application.
// Like env vars, mutations take effect on the next deploy.
type VolumeService struct {
	*BaseService
}

func NewVolumeService(deps *ServiceDeps) *VolumeService {
	return &VolumeService{BaseService: NewBaseService(deps)}
}

func (s *VolumeService) ListVolumes(
	ctx context.Context, applicationID, projectID, serverID, teamID string,
) ([]dto.VolumeResponse, error) {
	if _, err := s.scopedAppForVolume(ctx, applicationID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Volume().ListForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VolumeResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *dto.ToVolumeResponse(&rows[i]))
	}
	return out, nil
}

func (s *VolumeService) CreateVolume(
	ctx context.Context, applicationID, projectID, serverID, teamID, userID string,
	req *dto.CreateVolumeRequest,
) (dto.VolumeResponse, error) {
	_ = userID
	app, err := s.scopedAppForVolume(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.VolumeResponse{}, err
	}

	name := strings.TrimSpace(req.Name)
	if !validVolumeName.MatchString(name) {
		return dto.VolumeResponse{}, fiberutil.BadRequest(
			"Volume name must be lowercase letters, digits, or hyphens",
		)
	}
	mountPath := strings.TrimSpace(req.MountPath)
	if !strings.HasPrefix(mountPath, "/") {
		return dto.VolumeResponse{}, fiberutil.BadRequest("Mount path must be absolute")
	}

	if req.Type == "bind" {
		if req.HostPath == nil || strings.TrimSpace(*req.HostPath) == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Bind volumes require host_path")
		}
		if !strings.HasPrefix(*req.HostPath, "/") {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Host path must be absolute")
		}
	}

	taken, err := s.Repos().Volume().ExistsByName(ctx, applicationID, name, "")
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	if taken {
		return dto.VolumeResponse{}, fiberutil.Conflict(
			"A volume with that name already exists on this application",
		)
	}

	v := &models.ApplicationVolume{
		ApplicationID: applicationID,
		Name:          name,
		MountPath:     mountPath,
		Type:          req.Type,
		HostPath:      req.HostPath,
	}
	if err := s.Repos().Volume().Create(ctx, v); err != nil {
		return dto.VolumeResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.volume.added", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"volume_id":      v.ID,
	})
	return *dto.ToVolumeResponse(v), nil
}

func (s *VolumeService) UpdateVolume(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
	req *dto.UpdateVolumeRequest,
) (dto.VolumeResponse, error) {
	_ = userID
	app, err := s.scopedAppForVolume(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	v, err := s.Repos().Volume().FindByID(ctx, id)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	if v.ApplicationID != applicationID {
		return dto.VolumeResponse{}, fiberutil.NotFound()
	}

	updates := map[string]any{}
	if req.MountPath != nil {
		mp := strings.TrimSpace(*req.MountPath)
		if !strings.HasPrefix(mp, "/") {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Mount path must be absolute")
		}
		updates["mount_path"] = mp
	}
	if req.HostPath != nil {
		hp := strings.TrimSpace(*req.HostPath)
		if v.Type == "bind" && !strings.HasPrefix(hp, "/") {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Host path must be absolute for bind volumes")
		}
		if hp == "" {
			updates["host_path"] = nil
		} else {
			updates["host_path"] = hp
		}
	}
	if len(updates) > 0 {
		if err := s.Repos().Volume().UpdateFields(ctx, id, updates); err != nil {
			return dto.VolumeResponse{}, err
		}
	}
	reloaded, err := s.Repos().Volume().FindByID(ctx, id)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.application.volume.updated", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"volume_id":      id,
	})
	return *dto.ToVolumeResponse(reloaded), nil
}

func (s *VolumeService) DeleteVolume(
	ctx context.Context, id, applicationID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	app, err := s.scopedAppForVolume(ctx, applicationID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	v, err := s.Repos().Volume().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ApplicationID != applicationID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Volume().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.application.volume.deleted", map[string]any{
		"application_id": app.ID,
		"server_id":      app.ServerID,
		"team_id":        app.TeamID,
		"volume_id":      id,
	})
	return nil
}

func (s *VolumeService) scopedAppForVolume(
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

// validVolumeName: docker's allowed pattern for named volumes.
var validVolumeName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
