package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
)

const (
	PHPConfigurationKindINI = "php_ini"
	PHPConfigurationKindFPM = "php_fpm"
	phpConfigurationMaxSize = 256 * 1024
)

type phpConfigurationTarget struct {
	Kind    string
	Label   string
	Path    string
	Version string
}

func phpConfigurationForService(service *models.InstalledService, kind string) (phpConfigurationTarget, error) {
	if service.Type != types.ServiceTypePhp {
		return phpConfigurationTarget{}, fiberutil.NotFound("PHP service not found")
	}

	version := types.Software(service.Software).GetVersion()
	switch kind {
	case PHPConfigurationKindINI:
		return phpConfigurationTarget{
			Kind:    kind,
			Label:   "php.ini",
			Path:    paths.PHPIni(version),
			Version: version,
		}, nil
	case PHPConfigurationKindFPM:
		return phpConfigurationTarget{
			Kind:    kind,
			Label:   "php-fpm.conf",
			Path:    paths.PHPFPMConfig(version),
			Version: version,
		}, nil
	default:
		return phpConfigurationTarget{}, fiberutil.BadRequest("Unsupported PHP configuration file")
	}
}

func (s *Service) phpConfigurationContext(ctx context.Context, serverID, teamID, phpID, kind string) (*models.Server, phpConfigurationTarget, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, phpConfigurationTarget{}, err
	}

	service, err := s.repos.Service().FindByID(ctx, phpID)
	if err != nil {
		return nil, phpConfigurationTarget{}, fiberutil.NotFoundAs(err, "PHP service not found")
	}
	if service.ServerID != serverID {
		return nil, phpConfigurationTarget{}, fiberutil.NotFound("PHP service not found")
	}

	target, err := phpConfigurationForService(service, kind)
	if err != nil {
		return nil, phpConfigurationTarget{}, err
	}
	return server, target, nil
}

// GetPHPConfiguration returns one trusted, version-scoped PHP configuration
// file from the server.
func (s *Service) GetPHPConfiguration(ctx context.Context, serverID, teamID, phpID, kind string) (*dto.PHPConfigurationResponse, error) {
	server, target, err := s.phpConfigurationContext(ctx, serverID, teamID, phpID, kind)
	if err != nil {
		return nil, err
	}

	task := tasks.ReadPHPConfiguration(tasks.ReadPHPConfigurationConfig{
		Path:     target.Path,
		MaxBytes: phpConfigurationMaxSize,
	})
	result, err := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger).
		AsRoot().
		Run(ctx)
	if err != nil || result == nil || !result.IsSuccessful() {
		return nil, fiberutil.Internal("Unable to read PHP configuration")
	}

	return &dto.PHPConfigurationResponse{
		Kind:     target.Kind,
		Label:    target.Label,
		Path:     target.Path,
		Contents: result.GetOutput(),
	}, nil
}

// UpdatePHPConfiguration validates and writes one trusted, version-scoped PHP
// configuration file, then reloads PHP-FPM. The remote task restores the prior
// file automatically if the reload fails.
func (s *Service) UpdatePHPConfiguration(ctx context.Context, serverID, teamID, phpID, kind string, req *dto.UpdatePHPConfigurationRequest) (*dto.PHPConfigurationResponse, error) {
	server, target, err := s.phpConfigurationContext(ctx, serverID, teamID, phpID, kind)
	if err != nil {
		return nil, err
	}

	task := tasks.UpdatePHPConfiguration(tasks.UpdatePHPConfigurationConfig{
		Kind:     target.Kind,
		Version:  target.Version,
		Path:     target.Path,
		Contents: req.Contents,
	})
	result, err := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger).
		AsRoot().
		Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update PHP configuration: %w", err)
	}
	if result == nil || !result.IsSuccessful() {
		output := ""
		if result != nil {
			output = result.GetOutput()
		}
		if strings.Contains(output, tasks.PHPConfigValidationFailedMarker) {
			return nil, fiberutil.Validation("PHP configuration validation failed. Review the file for syntax errors.")
		}
		if strings.Contains(output, tasks.PHPConfigReloadFailedMarker) {
			return nil, fiberutil.Internal("PHP-FPM could not reload. The previous configuration was restored.")
		}
		return nil, fiberutil.Internal("Unable to update PHP configuration")
	}

	return &dto.PHPConfigurationResponse{
		Kind:     target.Kind,
		Label:    target.Label,
		Path:     target.Path,
		Contents: req.Contents,
	}, nil
}
