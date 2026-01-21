package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
)

// ListServices returns all services for a server
func (s *Service) ListServices(ctx context.Context, serverID, teamID string) ([]models.InstalledService, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	services, err := s.repos.Service().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	result := make([]models.InstalledService, len(services))
	copy(result, services)

	return result, nil
}

// InstallService installs a software on a server
func (s *Service) InstallService(ctx context.Context, serverID, teamID string, req *dto.CreateServiceRequest) (*models.InstalledService, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	software, err := enums.ParseSoftware(req.Software)
	if err != nil {
		return nil, ErrInvalidSoftware
	}

	existingService, err := s.repos.Service().FindByServerAndSoftware(ctx, serverID, software)
	if err != nil && !errors.Is(err, repositories.ErrServiceNotFound) {
		return nil, fmt.Errorf("failed to check existing service: %w", err)
	}
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

	if err := s.repos.Service().Create(ctx, service); err != nil {
		return nil, err
	}

	if err := s.dispatchServiceInstallJob(server, service); err != nil {
		s.LogError(err, "Failed to dispatch service install job", "server_id", serverID, "service_id", service.ID)
	}

	return service, nil
}

// HandleServiceOperation handles service operations (start, stop, restart, remove, status)
func (s *Service) HandleServiceOperation(ctx context.Context, serverID, teamID, serviceID string, operation enums.ServiceOption) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
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
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
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
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
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
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// Get all installed services for this server
	installedServices, err := s.repos.Service().FindByServer(ctx, serverID)
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
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// Get all installed PHP services
	installedServices, err := s.repos.Service().FindByServerAndType(ctx, serverID, enums.ServiceTypePhp)
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

// GetInstalledPhpVersions returns only the installed PHP versions for a server (simplified for dropdowns)
func (s *Service) GetInstalledPhpVersions(ctx context.Context, serverID, teamID string) ([]dto.InstalledPhpVersionResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	// Get all installed PHP services
	installedServices, err := s.repos.Service().FindByServerAndType(ctx, serverID, enums.ServiceTypePhp)
	if err != nil {
		return nil, err
	}

	// Find the default PHP version
	var defaultID string
	for _, svc := range installedServices {
		if svc.IsDefault {
			defaultID = svc.ID
			break
		}
	}

	// If no default is set, use the first active one
	if defaultID == "" {
		for _, svc := range installedServices {
			if svc.Status.IsActive() {
				defaultID = svc.ID
				break
			}
		}
	}

	result := make([]dto.InstalledPhpVersionResponse, 0, len(installedServices))
	for _, svc := range installedServices {
		sw := enums.Software(svc.Software)
		result = append(result, dto.InstalledPhpVersionResponse{
			ID:          svc.ID,
			Key:         svc.Software,
			DisplayName: sw.Label(),
			Version:     svc.Version,
			IsDefault:   svc.ID == defaultID,
		})
	}

	// Sort by version descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return compareVersions(result[i].Version, result[j].Version) > 0
	})

	return result, nil
}

// compareVersions compares two version strings (e.g., "8.2" vs "8.1")
// Returns positive if v1 > v2, negative if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 != n2 {
			return n1 - n2
		}
	}
	return 0
}

// SetDefaultPhpVersion sets the default PHP version for a server
func (s *Service) SetDefaultPhpVersion(ctx context.Context, serverID, teamID, serviceID string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return ErrServiceNotFound
	}

	if service.Type != enums.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP service")
	}

	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewSetDefaultPhpTask(server.ID, service.ID, service.Version, userID)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// InstallPhpExtension installs a PHP extension on a server
func (s *Service) InstallPhpExtension(ctx context.Context, serverID, teamID, version, extension string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewInstallPhpExtensionTask(server.ID, version, extension, userID)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// UninstallPhpExtension uninstalls a PHP extension from a server
func (s *Service) UninstallPhpExtension(ctx context.Context, serverID, teamID, version, extension string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewUninstallPhpExtensionTask(server.ID, version, extension, userID)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}
