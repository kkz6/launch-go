package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

// ListServices returns all services for a server. Signature matches
// IndexNestedFunc.
func (s *Service) ListServices(ctx context.Context, serverID, teamID string) ([]dto.ServiceResponse, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	svcs, err := s.repos.Service().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ServiceResponse, len(svcs))
	for i := range svcs {
		out[i] = dto.ToServiceResponse(&svcs[i])
		out[i].DefaultPending = server.PendingDefaultPHPServiceID != nil &&
			*server.PendingDefaultPHPServiceID == svcs[i].ID
	}
	return out, nil
}

// InstallService installs a software on a server. Signature matches
// CreateNestedFunc.
func (s *Service) InstallService(ctx context.Context, serverID, teamID, userID string, req *dto.CreateServiceRequest) (dto.ServiceResponse, error) {
	_ = userID
	svc, err := s.runInstallService(ctx, serverID, teamID, req)
	if err != nil {
		return dto.ServiceResponse{}, err
	}
	return dto.ToServiceResponse(svc), nil
}

func (s *Service) runInstallService(ctx context.Context, serverID, teamID string, req *dto.CreateServiceRequest) (*models.InstalledService, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	software, err := types.ParseSoftware(req.Software)
	if err != nil {
		return nil, ErrInvalidSoftware
	}

	existingService, err := s.repos.Service().FindByServerAndSoftware(ctx, serverID, software)
	if err != nil && !fiberutil.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing service: %w", err)
	}
	if existingService != nil {
		return nil, ErrServiceAlreadyExists
	}

	service := &models.InstalledService{
		Type:      software.GetServiceType(),
		Name:      software.Label(),
		Software:  software.String(),
		Status:    types.ServiceStatusPending,
		IsDefault: false,
		Version:   software.GetVersion(),
	}
	service.ServerID = serverID

	if err := s.repos.Service().Create(ctx, service); err != nil {
		return nil, err
	}

	if err := s.dispatchServiceInstallJob(server, service); err != nil {
		s.LogError(err, "Failed to dispatch service install job", "server_id", serverID, "service_id", service.ID)
	}

	return service, nil
}

// HandleServiceOperation handles service operations (start, stop, restart, remove, status)
func (s *Service) HandleServiceOperation(ctx context.Context, serverID, teamID, serviceID string, operation types.ServiceOption) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return fiberutil.NotFound()
	}

	if service.Status == types.ServiceStatusUpdating {
		return ErrServiceBusy
	}

	switch operation {
	case types.ServiceOptionStart:
		return s.dispatchServiceOperationJob(ctx, server, service, "start")
	case types.ServiceOptionRestart:
		return s.dispatchServiceOperationJob(ctx, server, service, "restart")
	case types.ServiceOptionStop:
		return s.dispatchServiceOperationJob(ctx, server, service, "stop")
	case types.ServiceOptionRemove:
		return s.dispatchServiceRemoveJob(ctx, server, service)
	case types.ServiceOptionStatus:
		return s.dispatchServiceStatusJob(server, service)
	case types.ServiceOptionUpdate:
		// In-place binary upgrade via the service-operation job — NOT the
		// full install, which rewrites config/systemd and needs template
		// vars the generic install path doesn't populate. For the Launch
		// Agent this re-runs the installer's binary swap + restart.
		return s.dispatchServiceOperationJob(ctx, server, service, "update")
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

func (s *Service) dispatchServiceInstallJob(server *models.Server, svc *models.InstalledService) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewAddServiceTask(server.ID, svc.ID, svc.Software)
	})
}

func (s *Service) dispatchServiceOperationJob(
	ctx context.Context,
	server *models.Server,
	svc *models.InstalledService,
	operation string,
) error {
	previousStatus, reserved, err := s.reservePHPServiceLifecycle(ctx, svc)
	if err != nil {
		return err
	}

	dispatchErr := s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewServiceOperationTask(
			server.ID,
			svc.ID,
			operation,
			previousStatus,
			nil,
		)
	})
	if dispatchErr != nil && reserved {
		return s.rollbackPHPServiceDispatch(ctx, svc, previousStatus, dispatchErr)
	}
	return dispatchErr
}

func (s *Service) dispatchServiceRemoveJob(
	ctx context.Context,
	server *models.Server,
	svc *models.InstalledService,
) error {
	previousStatus, reserved, err := s.reservePHPServiceLifecycle(ctx, svc)
	if err != nil {
		return err
	}

	dispatchErr := s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewRemoveServiceTask(
			server.ID,
			svc.ID,
			previousStatus,
			nil,
		)
	})
	if dispatchErr != nil && reserved {
		return s.rollbackPHPServiceDispatch(ctx, svc, previousStatus, dispatchErr)
	}
	return dispatchErr
}

func (s *Service) reservePHPServiceLifecycle(
	ctx context.Context,
	svc *models.InstalledService,
) (types.ServiceStatus, bool, error) {
	if svc.Type != types.ServiceTypePhp || !svc.Status.IsActive() {
		return "", false, nil
	}
	if !s.HasQueue() {
		return "", false, pkgservice.ErrQueueRequired
	}

	previousStatus := svc.Status
	claimed, err := s.repos.Service().ClaimPhpPatch(ctx, svc.ID, previousStatus)
	if err != nil {
		return "", false, fmt.Errorf("reserve PHP service operation: %w", err)
	}
	if !claimed {
		return "", false, ErrServiceBusy
	}
	svc.Status = types.ServiceStatusUpdating
	return previousStatus, true, nil
}

func (s *Service) rollbackPHPServiceDispatch(
	ctx context.Context,
	svc *models.InstalledService,
	previousStatus types.ServiceStatus,
	dispatchErr error,
) error {
	restored, restoreErr := s.restorePhpPatchReservation(ctx, svc.ID, previousStatus)
	if restoreErr != nil {
		return fmt.Errorf(
			"queue PHP service operation: %w (restore status: %v)",
			dispatchErr,
			restoreErr,
		)
	}
	if !restored {
		return fmt.Errorf(
			"queue PHP service operation: %w (reservation could not be released)",
			dispatchErr,
		)
	}
	svc.Status = previousStatus
	return dispatchErr
}

func (s *Service) dispatchServiceStatusJob(server *models.Server, svc *models.InstalledService) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewCheckServiceStatusTask(server.ID, svc.ID, nil)
	})
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
		return nil, fiberutil.NotFound()
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
		return fiberutil.NotFound()
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
	var installedDbType types.ServiceType
	for i := range installedServices {
		svc := &installedServices[i]
		installedMap[svc.Software] = svc
		// Track which database type is installed
		if svc.Type.IsDatabase() {
			installedDbType = svc.Type
		}
	}

	// Get all software groups
	groups := types.GetAllSoftwareGroups()
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
	installedServices, err := s.repos.Service().FindByServerAndType(ctx, serverID, types.ServiceTypePhp)
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
	allPhpVersions := types.AllPhpVersions()
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
	installedServices, err := s.repos.Service().FindByServerAndType(ctx, serverID, types.ServiceTypePhp)
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
		sw := types.Software(svc.Software)
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

// SetDefaultPhpVersion sets the default PHP version for a server. Its
// parameter order matches fiber.ActionItemNestedFunc.
func (s *Service) SetDefaultPhpVersion(ctx context.Context, serviceID, serverID, teamID, userID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return fiberutil.NotFound()
	}

	if service.Type != types.ServiceTypePhp {
		return fiberutil.BadRequest("Service is not a PHP installation")
	}

	software := service.GetSoftware()
	if !software.IsPhp() {
		return fiberutil.BadRequest("Service has an invalid PHP software identity")
	}
	if service.Status == types.ServiceStatusUpdating {
		return ErrServiceBusy
	}
	if !service.Status.IsActive() {
		return fiberutil.BadRequest("PHP service must be active to become the default")
	}
	if service.IsDefault {
		return nil
	}
	if !s.HasQueue() {
		return pkgservice.ErrQueueRequired
	}
	if s.repos.DB() == nil {
		return fmt.Errorf("database not configured")
	}

	previousStatus := service.Status
	if err := s.reserveDefaultPHPChange(
		ctx,
		server.ID,
		service.ID,
		previousStatus,
	); err != nil {
		return err
	}
	service.Status = types.ServiceStatusUpdating

	task, err := jobs.NewSetDefaultPhpTask(
		server.ID,
		service.ID,
		software.GetVersion(),
		previousStatus,
		&userID,
	)
	if err == nil {
		_, err = s.Queue.EnqueueDefault(task)
	}
	if err != nil {
		restored, releaseErr := s.releaseDefaultPHPReservation(
			ctx,
			server.ID,
			service.ID,
			previousStatus,
		)
		if releaseErr != nil {
			return fmt.Errorf("queue default PHP change: %w (release reservation: %v)", err, releaseErr)
		}
		if !restored {
			return fmt.Errorf(
				"queue default PHP change: %w (target service reservation was lost)",
				err,
			)
		}
		service.Status = previousStatus
		return err
	}

	s.BroadcastToTeam(teamID, "php.default_change", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"status":     "queued",
		"version":    software.GetVersion(),
	})
	return nil
}

func (s *Service) reserveDefaultPHPChange(
	ctx context.Context,
	serverID,
	serviceID string,
	previousStatus types.ServiceStatus,
) error {
	if !previousStatus.IsActive() {
		return fiberutil.BadRequest("PHP service must be active to become the default")
	}

	err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		target := tx.Model(&models.InstalledService{}).
			Where(
				"id = ? AND server_id = ? AND type = ? AND status = ?",
				serviceID,
				serverID,
				types.ServiceTypePhp,
				previousStatus,
			).
			Update("status", types.ServiceStatusUpdating)
		if target.Error != nil {
			return target.Error
		}
		if target.RowsAffected != 1 {
			return ErrServiceBusy
		}

		reservation := tx.Model(&models.Server{}).
			Where("id = ? AND pending_default_php_service_id IS NULL", serverID).
			Update("pending_default_php_service_id", serviceID)
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return ErrServiceBusy
		}
		return nil
	})
	if err != nil {
		if err == ErrServiceBusy {
			return err
		}
		return fmt.Errorf("reserve default PHP change: %w", err)
	}
	return nil
}

func (s *Service) releaseDefaultPHPReservation(
	ctx context.Context,
	serverID,
	serviceID string,
	previousStatus types.ServiceStatus,
) (bool, error) {
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()

	restored := false
	err := s.repos.DB().WithContext(cleanupCtx).Transaction(func(tx *gorm.DB) error {
		reservation := tx.Model(&models.Server{}).
			Where(
				"id = ? AND pending_default_php_service_id = ?",
				serverID,
				serviceID,
			).
			Update("pending_default_php_service_id", nil)
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return fmt.Errorf("default PHP reservation could not be released")
		}

		target := tx.Model(&models.InstalledService{}).
			Where(
				"id = ? AND server_id = ? AND type = ? AND status = ?",
				serviceID,
				serverID,
				types.ServiceTypePhp,
				types.ServiceStatusUpdating,
			).
			Update("status", previousStatus)
		if target.Error != nil {
			return target.Error
		}
		restored = target.RowsAffected == 1
		return nil
	})
	return restored, err
}

// PatchPhpVersion queues an in-place patch of an installed PHP major.minor
// series. Its parameter order matches fiber.ActionItemNestedFunc.
func (s *Service) PatchPhpVersion(ctx context.Context, serviceID, serverID, teamID, userID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	service, err := s.repos.Service().FindByID(ctx, serviceID)
	if err != nil {
		return err
	}

	if service.ServerID != serverID {
		return fiberutil.NotFound()
	}

	if service.Type != types.ServiceTypePhp || !service.GetSoftware().IsPhp() {
		return fiberutil.BadRequest("Service is not a PHP installation")
	}
	if service.Status == types.ServiceStatusUpdating {
		return ErrServiceBusy
	}
	if !service.Status.IsActive() {
		return fiberutil.BadRequest("PHP service must be active before patching")
	}
	if !s.HasQueue() {
		return pkgservice.ErrQueueRequired
	}

	previousStatus := service.Status
	claimed, err := s.repos.Service().ClaimPhpPatch(ctx, service.ID, previousStatus)
	if err != nil {
		return fmt.Errorf("reserve PHP patch: %w", err)
	}
	if !claimed {
		if service.Status == types.ServiceStatusUpdating || previousStatus.IsActive() {
			return ErrServiceBusy
		}
		return fiberutil.BadRequest("PHP service must be active before patching")
	}
	queuedTypeData := make(dbtype.JSONMap, len(service.TypeData)+1)
	for key, value := range service.TypeData {
		queuedTypeData[key] = value
	}
	queuedTypeData["patch_status"] = "queued"
	delete(queuedTypeData, "patch_error")
	delete(queuedTypeData, "patch_finished_at")
	if err := s.repos.Service().UpdateFields(ctx, service.ID, map[string]any{
		"type_data": queuedTypeData,
		"task_id":   nil,
	}); err != nil {
		restored, rollbackErr := s.restorePhpPatchReservation(
			ctx,
			service.ID,
			previousStatus,
		)
		if rollbackErr != nil {
			return fmt.Errorf(
				"initialize PHP patch state: %w (rollback status: %v)",
				err,
				rollbackErr,
			)
		}
		if !restored {
			return fmt.Errorf(
				"initialize PHP patch state: %w (patch reservation could not be released)",
				err,
			)
		}
		return fmt.Errorf("initialize PHP patch state: %w", err)
	}
	service.TypeData = queuedTypeData
	service.TaskID = nil
	s.BroadcastToTeam(teamID, "service.status_changed", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"status":     types.ServiceStatusUpdating.String(),
	})
	s.BroadcastToTeam(teamID, "php.patch", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"status":     "queued",
		"version":    service.PhpVersionSeries(),
	})

	task, dispatchErr := jobs.NewPatchPhpVersionTask(
		server.ID,
		service.ID,
		previousStatus,
		&userID,
	)
	if dispatchErr == nil {
		_, dispatchErr = s.Queue.EnqueueDefault(task)
	}
	if dispatchErr == nil {
		return nil
	}

	restored, rollbackErr := s.restorePhpPatchReservation(
		ctx,
		service.ID,
		previousStatus,
	)
	if rollbackErr != nil {
		return fmt.Errorf("queue PHP patch: %w (rollback status: %v)", dispatchErr, rollbackErr)
	}
	if !restored {
		return fmt.Errorf("queue PHP patch: %w (patch reservation could not be released)", dispatchErr)
	}
	s.BroadcastToTeam(teamID, "service.status_changed", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"status":     previousStatus.String(),
	})
	s.BroadcastToTeam(teamID, "php.patch", map[string]any{
		"server_id":  server.ID,
		"service_id": service.ID,
		"status":     "failed",
		"version":    service.PhpVersionSeries(),
		"output":     dispatchErr.Error(),
	})
	return dispatchErr
}

func (s *Service) restorePhpPatchReservation(
	ctx context.Context,
	serviceID string,
	previousStatus types.ServiceStatus,
) (bool, error) {
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	return s.repos.Service().RestorePhpPatchStatus(
		cleanupCtx,
		serviceID,
		previousStatus,
	)
}

// InstallPhpExtension installs a PHP extension on a server
func (s *Service) InstallPhpExtension(ctx context.Context, serverID, teamID, version, extension string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	phpService, err := s.repos.Service().FindPhpByServerAndVersion(ctx, serverID, version)
	if err != nil {
		return err
	}
	if phpService.Status == types.ServiceStatusUpdating {
		return ErrServiceBusy
	}
	if !phpService.Status.IsActive() {
		return fiberutil.BadRequest("PHP service must be active to install extensions")
	}
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}
	_ = s.repos.Service().SetExtensionStatus(ctx, phpService.ID, extension, "installing")

	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewInstallPhpExtensionTask(server.ID, version, extension, userID)
	})
}

// UninstallPhpExtension uninstalls a PHP extension from a server
func (s *Service) UninstallPhpExtension(ctx context.Context, serverID, teamID, version, extension string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	phpService, err := s.repos.Service().FindPhpByServerAndVersion(ctx, serverID, version)
	if err != nil {
		return err
	}
	if phpService.Status == types.ServiceStatusUpdating {
		return ErrServiceBusy
	}
	if !phpService.Status.IsActive() {
		return fiberutil.BadRequest("PHP service must be active to remove extensions")
	}
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}
	_ = s.repos.Service().SetExtensionStatus(ctx, phpService.ID, extension, "removing")

	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewUninstallPhpExtensionTask(server.ID, version, extension, userID)
	})
}
