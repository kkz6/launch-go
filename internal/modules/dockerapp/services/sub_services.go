package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// requireApp loads an app for a (id, server, team) tuple.
func (s *Service) requireApp(ctx context.Context, appID, serverID, teamID string) (*models.App, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	return s.repos.App().FindByIDAndServer(ctx, appID, serverID)
}

// ----- Env Vars -----

// CreateEnvVar adds an env var to an app.
// Signature matches CreateDoubleNestedFunc.
func (s *Service) CreateEnvVar(ctx context.Context, appID, serverID, teamID, userID string, req *dto.CreateEnvVarRequest) (dto.EnvVarResponse, error) {
	_ = userID
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return dto.EnvVarResponse{}, err
	}
	e := &models.EnvVar{
		AppID:  app.ID,
		Key:    req.Key,
		Value:  dbtype.EncryptedString(req.Value),
		Secret: req.Secret,
	}
	if err := s.repos.EnvVar().Create(ctx, e); err != nil {
		return dto.EnvVarResponse{}, fmt.Errorf("failed to create env var: %w", err)
	}
	return projectEnvVar(e), nil
}

// ListEnvVars returns env vars for an app.
// Signature matches IndexDoubleNestedFunc.
func (s *Service) ListEnvVars(ctx context.Context, appID, serverID, teamID string) ([]dto.EnvVarResponse, error) {
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repos.EnvVar().FindByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EnvVarResponse, len(rows))
	for i := range rows {
		out[i] = projectEnvVar(&rows[i])
	}
	return out, nil
}

// DeleteEnvVar removes an env var by ID.
// Signature matches DeleteDoubleNestedFunc.
func (s *Service) DeleteEnvVar(ctx context.Context, envVarID, appID, serverID, teamID, userID string) error {
	_ = userID
	if _, err := s.requireApp(ctx, appID, serverID, teamID); err != nil {
		return err
	}
	return s.repos.EnvVar().Delete(ctx, envVarID)
}

func projectEnvVar(e *models.EnvVar) dto.EnvVarResponse {
	out := dto.EnvVarResponse{ID: e.ID, Key: e.Key, Secret: e.Secret}
	if !e.Secret {
		out.Value = e.Value.String()
	}
	return out
}

// ----- Ports -----

// CreatePort publishes a port for an app.
func (s *Service) CreatePort(ctx context.Context, appID, serverID, teamID, userID string, req *dto.CreatePortRequest) (dto.PortResponse, error) {
	_ = userID
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return dto.PortResponse{}, err
	}
	proto := req.Protocol
	if proto == "" {
		proto = "tcp"
	}
	p := &models.Port{
		AppID:         app.ID,
		HostPort:      req.HostPort,
		ContainerPort: req.ContainerPort,
		Protocol:      proto,
	}
	if err := s.repos.Port().Create(ctx, p); err != nil {
		return dto.PortResponse{}, fmt.Errorf("failed to create port: %w", err)
	}
	return dto.PortResponse{
		ID: p.ID, HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol,
	}, nil
}

// ListPorts returns ports for an app.
func (s *Service) ListPorts(ctx context.Context, appID, serverID, teamID string) ([]dto.PortResponse, error) {
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repos.Port().FindByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PortResponse, len(rows))
	for i, p := range rows {
		out[i] = dto.PortResponse{
			ID: p.ID, HostPort: p.HostPort, ContainerPort: p.ContainerPort, Protocol: p.Protocol,
		}
	}
	return out, nil
}

// DeletePort removes a port row.
func (s *Service) DeletePort(ctx context.Context, portID, appID, serverID, teamID, userID string) error {
	_ = userID
	if _, err := s.requireApp(ctx, appID, serverID, teamID); err != nil {
		return err
	}
	return s.repos.Port().Delete(ctx, portID)
}

// ----- Volumes -----

// CreateVolume adds a named volume mount.
func (s *Service) CreateVolume(ctx context.Context, appID, serverID, teamID, userID string, req *dto.CreateVolumeRequest) (dto.VolumeResponse, error) {
	_ = userID
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return dto.VolumeResponse{}, err
	}
	v := &models.Volume{
		AppID:     app.ID,
		Name:      req.Name,
		MountPath: req.MountPath,
	}
	if err := s.repos.Volume().Create(ctx, v); err != nil {
		return dto.VolumeResponse{}, fmt.Errorf("failed to create volume: %w", err)
	}
	return dto.ToVolumeResponse(app, v), nil
}

// ListVolumes returns volumes for an app.
func (s *Service) ListVolumes(ctx context.Context, appID, serverID, teamID string) ([]dto.VolumeResponse, error) {
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repos.Volume().FindByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VolumeResponse, len(rows))
	for i := range rows {
		out[i] = dto.ToVolumeResponse(app, &rows[i])
	}
	return out, nil
}

// DeleteVolume removes a volume row.
func (s *Service) DeleteVolume(ctx context.Context, volumeID, appID, serverID, teamID, userID string) error {
	_ = userID
	if _, err := s.requireApp(ctx, appID, serverID, teamID); err != nil {
		return err
	}
	return s.repos.Volume().Delete(ctx, volumeID)
}

// ----- Domains -----

// CreateDomain registers a domain for an app.
func (s *Service) CreateDomain(ctx context.Context, appID, serverID, teamID, userID string, req *dto.CreateDomainRequest) (dto.DomainResponse, error) {
	_ = userID
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return dto.DomainResponse{}, err
	}
	if existing, err := s.repos.Domain().FindByDomain(ctx, req.Domain); err != nil {
		return dto.DomainResponse{}, err
	} else if existing != nil {
		return dto.DomainResponse{}, ErrDomainTaken
	}
	d := &models.Domain{
		AppID:         app.ID,
		Domain:        req.Domain,
		ContainerPort: req.ContainerPort,
		TLS:           req.TLS,
	}
	if err := s.repos.Domain().Create(ctx, d); err != nil {
		return dto.DomainResponse{}, fmt.Errorf("failed to create domain: %w", err)
	}
	return dto.DomainResponse{
		ID: d.ID, Domain: d.Domain, ContainerPort: d.ContainerPort, TLS: d.TLS,
	}, nil
}

// ListDomains returns domains for an app.
func (s *Service) ListDomains(ctx context.Context, appID, serverID, teamID string) ([]dto.DomainResponse, error) {
	app, err := s.requireApp(ctx, appID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repos.Domain().FindByApp(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DomainResponse, len(rows))
	for i, d := range rows {
		out[i] = dto.DomainResponse{
			ID: d.ID, Domain: d.Domain, ContainerPort: d.ContainerPort, TLS: d.TLS,
		}
	}
	return out, nil
}

// DeleteDomain removes a domain row.
func (s *Service) DeleteDomain(ctx context.Context, domainID, appID, serverID, teamID, userID string) error {
	_ = userID
	if _, err := s.requireApp(ctx, appID, serverID, teamID); err != nil {
		return err
	}
	return s.repos.Domain().Delete(ctx, domainID)
}
