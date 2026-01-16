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
		Software:  software.String(),
		Status:    enums.ServiceStatusPending,
		IsDefault: false,
		Version:   software.GetVersion(),
	}

	if err := s.repo.CreateService(ctx, service); err != nil {
		return nil, err
	}

	if err := s.dispatchServiceInstallJob(server, service); err != nil {
		s.LogError(err, "Failed to dispatch service install job", "server_id", serverID, "service_id", service.ID)
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
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewAddServiceTask(server.ID, service.ID, string(service.Software))
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchServiceRestartJob(server *models.Server, service *models.InstalledService) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "restart", nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchServiceStopJob(server *models.Server, service *models.InstalledService) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "stop", nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchServiceRemoveJob(server *models.Server, service *models.InstalledService) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewServiceOperationTask(server.ID, service.ID, "remove", nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchServiceStatusJob(server *models.Server, service *models.InstalledService) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCheckServiceStatusTask(server.ID, service.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// GetServiceStatus returns the current status of a service from the database
func (s *Service) GetServiceStatus(ctx context.Context, serverID, teamID, serviceID string) (*models.InstalledService, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	service, err := s.repo.FindServiceByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	if service.ServerID != serverID {
		return nil, ErrServiceNotFound
	}

	return service, nil
}

// CheckServiceStatus triggers a status check on the server for a service
func (s *Service) CheckServiceStatus(ctx context.Context, serverID, teamID, serviceID string) error {
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

	return s.dispatchServiceStatusJob(server, service)
}

// GetAvailableServices returns all available services grouped by type with installation status
func (s *Service) GetAvailableServices(ctx context.Context, serverID, teamID string) ([]dto.AvailableSoftwareResponse, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// Get all installed services for this server
	installedServices, err := s.repo.FindServicesByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Build a map of installed software
	installedMap := make(map[string]*models.InstalledService)
	var installedDbType enums.ServiceType
	for i := range installedServices {
		svc := &installedServices[i]
		installedMap[svc.Software] = svc
		// Track which database type is installed
		if svc.Type.IsDatabase() {
			installedDbType = svc.Type
		}
	}

	// Get all software groups
	groups := enums.GetAllSoftwareGroups()
	result := make([]dto.AvailableSoftwareResponse, 0, len(groups))

	for _, group := range groups {
		// Skip conflicting database types
		if installedDbType != "" && group.Type.IsDatabase() && group.Type != installedDbType {
			continue
		}

		// Build versions list
		versions := make([]dto.SoftwareVersionResponse, len(group.Software))
		groupInstalled := false
		var groupStatus *string

		for i, software := range group.Software {
			installedSvc := installedMap[software.String()]
			isInstalled := installedSvc != nil

			version := dto.SoftwareVersionResponse{
				Software:  software.String(),
				Label:     software.Label(),
				Version:   software.GetVersion(),
				Installed: isInstalled,
			}

			if installedSvc != nil {
				status := installedSvc.Status.String()
				version.Status = &status
				groupInstalled = true
				groupStatus = &status
			}

			versions[i] = version
		}

		resp := dto.AvailableSoftwareResponse{
			Group:      group.Group,
			Label:      group.Label,
			Type:       group.Type.String(),
			ImagePath:  group.ImagePath,
			Installed:  groupInstalled,
			Status:     groupStatus,
			Versions:   versions,
			HasStart:   group.HasStart,
			HasStop:    group.HasStop,
			HasRestart: group.HasRestart,
			HasRemove:  group.HasRemove,
			HasStatus:  group.HasStatus,
		}

		result = append(result, resp)
	}

	return result, nil
}

// GetPhpVersions returns all PHP versions with their installation status for a server
func (s *Service) GetPhpVersions(ctx context.Context, serverID, teamID string) ([]dto.PhpVersionResponse, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// Get all installed PHP services
	installedServices, err := s.repo.FindServicesByServerAndType(ctx, serverID, enums.ServiceTypePhp)
	if err != nil {
		return nil, err
	}

	// Build a map of installed PHP versions
	installedMap := make(map[string]*models.InstalledService)
	var defaultService *models.InstalledService
	for i := range installedServices {
		svc := &installedServices[i]
		installedMap[svc.Software] = svc
		if svc.IsDefault {
			defaultService = svc
		}
	}

	// If no default is set, use the first installed one
	if defaultService == nil && len(installedServices) > 0 {
		for i := range installedServices {
			if installedServices[i].Status.IsActive() {
				defaultService = &installedServices[i]
				break
			}
		}
	}

	// Get all available PHP versions
	allPhpVersions := enums.AllPhpVersions()
	result := make([]dto.PhpVersionResponse, len(allPhpVersions))

	for i, software := range allPhpVersions {
		installedSvc := installedMap[software.String()]
		isInstalled := installedSvc != nil
		isDefault := isInstalled && defaultService != nil && installedSvc.ID == defaultService.ID

		resp := dto.PhpVersionResponse{
			Key:         software.String(),
			DisplayName: software.Label(),
			Version:     software.GetVersion(),
			IsInstalled: isInstalled,
			IsDefault:   isDefault,
		}

		if installedSvc != nil {
			svcResp := dto.ToServiceResponse(installedSvc)
			resp.Details = &svcResp
		}

		result[i] = resp
	}

	return result, nil
}
