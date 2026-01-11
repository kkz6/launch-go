package services

import (
	"context"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListServices returns all services for a server
func (s *Service) ListServices(ctx context.Context, serverID, teamID string) ([]models.InstalledService, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	services, err := s.repo.FindServicesByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	result := make([]models.InstalledService, len(services))
	for i, svc := range services {
		result[i] = svc
	}

	return result, nil
}

// InstallService installs a software on a server
func (s *Service) InstallService(ctx context.Context, serverID, teamID string, req *dto.CreateServiceRequest) (*models.InstalledService, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	software, err := enums.ParseSoftware(req.Software)
	if err != nil {
		return nil, ErrInvalidSoftware
	}

	existingService, _ := s.repo.FindServiceByServerAndSoftware(ctx, serverID, software)
	if existingService != nil {
		return nil, ErrServiceAlreadyExists
	}

	service := &models.InstalledService{
		ServerID:  serverID,
		Type:      software.GetServiceType(),
		Name:      software.Label(),
		Software:  &software,
		Status:    enums.ServiceStatusPending,
		IsDefault: false,
	}

	version := software.GetVersion()
	service.Version = &version

	if err := s.repo.CreateService(ctx, service); err != nil {
		return nil, err
	}

	if err := s.dispatchServiceInstallJob(server, service); err != nil {
		s.logger.Error().Err(err).
			Str("server_id", serverID).
			Str("service_id", service.ID).
			Msg("Failed to dispatch service install job")
	}

	return service, nil
}

// HandleServiceOperation handles service operations (start, stop, restart, remove, status)
func (s *Service) HandleServiceOperation(ctx context.Context, serverID, teamID, serviceID string, operation enums.ServiceOption) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repo.FindServiceByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return ErrServiceNotFound
	}

	switch operation {
	case enums.ServiceOptionStart, enums.ServiceOptionRestart:
		return s.dispatchServiceRestartJob(server, service)
	case enums.ServiceOptionStop:
		return s.dispatchServiceStopJob(server, service)
	case enums.ServiceOptionRemove:
		return s.dispatchServiceRemoveJob(server, service)
	case enums.ServiceOptionStatus:
		return s.dispatchServiceStatusJob(server, service)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

func (s *Service) dispatchServiceInstallJob(server *models.Server, service *models.InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewInstallServiceTask(server.ID, service.ID, service.Software.String())
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchServiceRestartJob(server *models.Server, service *models.InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "restart")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchServiceStopJob(server *models.Server, service *models.InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "stop")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchServiceRemoveJob(server *models.Server, service *models.InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "remove")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchServiceStatusJob(server *models.Server, service *models.InstalledService) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "status")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}
