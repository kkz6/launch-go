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
	return mapResponseValues(rows, dto.ToVolumeResponse), nil
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

	// Per-flavour validation. "named" is the legacy spelling — coerce
	// to "volume" before persistence so downstream code only sees one
	// of the three canonical types.
	normalisedType := req.Type
	if normalisedType == "named" {
		normalisedType = "volume"
	}
	switch normalisedType {
	case "bind":
		if req.HostPath == nil || strings.TrimSpace(*req.HostPath) == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Bind mounts require host_path")
		}
		if !strings.HasPrefix(strings.TrimSpace(*req.HostPath), "/") {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Host path must be absolute")
		}
	case "volume":
		// `name` already validated above; no extra fields required.
	case "file":
		if req.FilePath == nil || strings.TrimSpace(*req.FilePath) == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest(
				"File mounts require file_path (the on-host filename)",
			)
		}
	default:
		return dto.VolumeResponse{}, fiberutil.BadRequest("Unsupported mount type")
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

	// Trim & nil-out fields not relevant to the chosen type. Keeps
	// the row tidy so the response shape doesn't leak stale values
	// from previous edits (the form lets the user switch types
	// before saving on first create — we wipe what doesn't belong).
	var hostPath *string
	if normalisedType == "bind" && req.HostPath != nil {
		hp := strings.TrimSpace(*req.HostPath)
		hostPath = &hp
	}
	var content *string
	var filePath *string
	if normalisedType == "file" {
		content = req.Content
		if req.FilePath != nil {
			fp := strings.TrimSpace(*req.FilePath)
			filePath = &fp
		}
	}

	// ApplicationID is now `*string` (polymorphic with ComposeID).
	// Take the address of a local so the row's owner column is set.
	ownerID := applicationID
	v := &models.ApplicationVolume{
		ApplicationID: &ownerID,
		Name:          name,
		MountPath:     mountPath,
		Type:          normalisedType,
		HostPath:      hostPath,
		Content:       content,
		FilePath:      filePath,
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
	if v.ApplicationID == nil || *v.ApplicationID != applicationID {
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
	// File-mount fields. Only meaningful for type=file rows; we still
	// accept them on other rows (silently apply) rather than rejecting
	// — keeps the PATCH semantics simple and lets the UI switch a row
	// to file-mode in a future migration without a separate endpoint.
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.FilePath != nil {
		fp := strings.TrimSpace(*req.FilePath)
		if v.Type == "file" && fp == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest(
				"File mounts require a non-empty file_path",
			)
		}
		if fp == "" {
			updates["file_path"] = nil
		} else {
			updates["file_path"] = fp
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
	if v.ApplicationID == nil || *v.ApplicationID != applicationID {
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

// --- Compose-scoped operations -------------------------------------
//
// The same persistence row (`ApplicationVolume`) backs both
// application-owned and compose-stack-owned mounts. The compose
// surface uses a parallel set of methods that look identical but
// query the `compose_id` column. Validation is shared (same name
// regex, same per-type field rules) — only the owner-resolution
// helpers and the broadcast scope differ.
//
// Compose mounts are interpreted in `tasks/deploy_compose.go`:
//   - bind / volume → operator must wire them into the YAML
//     themselves (we surface the values; we don't rewrite
//     `docker-compose.yml`)
//   - file          → deploy task writes content to
//     `${STACK_DIR}/files/<file_path>` before `docker compose up`
//     so YAML can reference it via `./files/<file_path>`.

func (s *VolumeService) ListComposeVolumes(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) ([]dto.VolumeResponse, error) {
	if _, err := s.scopedComposeForVolume(ctx, composeID, projectID, serverID, teamID); err != nil {
		return nil, err
	}
	rows, err := s.Repos().Volume().ListForCompose(ctx, composeID)
	if err != nil {
		return nil, err
	}
	return mapResponseValues(rows, dto.ToVolumeResponse), nil
}

func (s *VolumeService) CreateComposeVolume(
	ctx context.Context, composeID, projectID, serverID, teamID, userID string,
	req *dto.CreateVolumeRequest,
) (dto.VolumeResponse, error) {
	_ = userID
	c, err := s.scopedComposeForVolume(ctx, composeID, projectID, serverID, teamID)
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

	normalisedType := req.Type
	if normalisedType == "named" {
		normalisedType = "volume"
	}
	switch normalisedType {
	case "bind":
		if req.HostPath == nil || strings.TrimSpace(*req.HostPath) == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Bind mounts require host_path")
		}
		if !strings.HasPrefix(strings.TrimSpace(*req.HostPath), "/") {
			return dto.VolumeResponse{}, fiberutil.BadRequest("Host path must be absolute")
		}
	case "volume":
		// validated above
	case "file":
		if req.FilePath == nil || strings.TrimSpace(*req.FilePath) == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest(
				"File mounts require file_path (the on-host filename)",
			)
		}
	default:
		return dto.VolumeResponse{}, fiberutil.BadRequest("Unsupported mount type")
	}

	taken, err := s.Repos().Volume().ExistsByNameForCompose(ctx, composeID, name, "")
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	if taken {
		return dto.VolumeResponse{}, fiberutil.Conflict(
			"A volume with that name already exists on this stack",
		)
	}

	var hostPath *string
	if normalisedType == "bind" && req.HostPath != nil {
		hp := strings.TrimSpace(*req.HostPath)
		hostPath = &hp
	}
	var content *string
	var filePath *string
	if normalisedType == "file" {
		content = req.Content
		if req.FilePath != nil {
			fp := strings.TrimSpace(*req.FilePath)
			filePath = &fp
		}
	}

	ownerID := composeID
	v := &models.ApplicationVolume{
		ComposeID: &ownerID,
		Name:      name,
		MountPath: mountPath,
		Type:      normalisedType,
		HostPath:  hostPath,
		Content:   content,
		FilePath:  filePath,
	}
	if err := s.Repos().Volume().Create(ctx, v); err != nil {
		return dto.VolumeResponse{}, err
	}
	s.BroadcastToTeam(teamID, "docker.compose.volume.added", map[string]any{
		"compose_id": c.ID,
		"server_id":  c.ServerID,
		"team_id":    c.TeamID,
		"volume_id":  v.ID,
	})
	return *dto.ToVolumeResponse(v), nil
}

func (s *VolumeService) UpdateComposeVolume(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
	req *dto.UpdateVolumeRequest,
) (dto.VolumeResponse, error) {
	_ = userID
	c, err := s.scopedComposeForVolume(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	v, err := s.Repos().Volume().FindByID(ctx, id)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	if v.ComposeID == nil || *v.ComposeID != composeID {
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
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.FilePath != nil {
		fp := strings.TrimSpace(*req.FilePath)
		if v.Type == "file" && fp == "" {
			return dto.VolumeResponse{}, fiberutil.BadRequest(
				"File mounts require a non-empty file_path",
			)
		}
		if fp == "" {
			updates["file_path"] = nil
		} else {
			updates["file_path"] = fp
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
	s.BroadcastToTeam(teamID, "docker.compose.volume.updated", map[string]any{
		"compose_id": c.ID,
		"server_id":  c.ServerID,
		"team_id":    c.TeamID,
		"volume_id":  id,
	})
	return *dto.ToVolumeResponse(reloaded), nil
}

func (s *VolumeService) DeleteComposeVolume(
	ctx context.Context, id, composeID, projectID, serverID, teamID, userID string,
) error {
	_ = userID
	c, err := s.scopedComposeForVolume(ctx, composeID, projectID, serverID, teamID)
	if err != nil {
		return err
	}
	v, err := s.Repos().Volume().FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v.ComposeID == nil || *v.ComposeID != composeID {
		return fiberutil.NotFound()
	}
	if err := s.Repos().Volume().Delete(ctx, id); err != nil {
		return err
	}
	s.BroadcastToTeam(teamID, "docker.compose.volume.deleted", map[string]any{
		"compose_id": c.ID,
		"server_id":  c.ServerID,
		"team_id":    c.TeamID,
		"volume_id":  id,
	})
	return nil
}

func (s *VolumeService) scopedComposeForVolume(
	ctx context.Context, composeID, projectID, serverID, teamID string,
) (*models.Compose, error) {
	if _, err := s.Repos().Project().FindByIDAndTeamServer(ctx, projectID, teamID, serverID); err != nil {
		return nil, err
	}
	c, err := s.Repos().Compose().FindByIDAndTeamServer(ctx, composeID, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, fiberutil.NotFound()
	}
	return c, nil
}

// validVolumeName: docker's allowed pattern for named volumes.
var validVolumeName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
