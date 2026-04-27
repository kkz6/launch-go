package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/managedservice/dto"
	"github.com/kkz6/launch-go/internal/modules/managedservice/jobs"
	"github.com/kkz6/launch-go/internal/modules/managedservice/models"
	mstasks "github.com/kkz6/launch-go/internal/modules/managedservice/tasks"
	"github.com/kkz6/launch-go/internal/modules/managedservice/types"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ListByServer returns every managed service for a server.
func (s *Service) ListByServer(ctx context.Context, serverID, teamID string) ([]dto.ManagedServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	services, err := s.repos.ManagedService().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return dto.ToManagedServiceResponseList(services), nil
}

// Get returns a single managed service for a server+kind.
func (s *Service) Get(ctx context.Context, serverID, teamID string, kind types.Kind) (dto.ManagedServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.ManagedServiceResponse{}, err
	}
	ms, err := s.repos.ManagedService().FindByServerAndKind(ctx, serverID, kind)
	if err != nil {
		return dto.ManagedServiceResponse{}, err
	}
	if ms == nil {
		return dto.ManagedServiceResponse{}, nil
	}
	return dto.ToManagedServiceResponse(ms), nil
}

// Install creates the row and dispatches the install job.
func (s *Service) Install(ctx context.Context, serverID, teamID, userID string, req *dto.InstallManagedServiceRequest) (dto.ManagedServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.ManagedServiceResponse{}, err
	}

	if !req.Kind.IsValid() {
		return dto.ManagedServiceResponse{}, fmt.Errorf("invalid kind: %s", req.Kind)
	}

	existing, err := s.repos.ManagedService().FindByServerAndKind(ctx, serverID, req.Kind)
	if err != nil {
		return dto.ManagedServiceResponse{}, err
	}
	if existing != nil {
		return dto.ManagedServiceResponse{}, ErrAlreadyInstalled
	}

	ms := s.buildManagedService(serverID, teamID, req)

	if err := s.repos.ManagedService().Create(ctx, ms); err != nil {
		return dto.ManagedServiceResponse{}, fmt.Errorf("failed to create managed service: %w", err)
	}

	activity.RecordEvent(ctx, "installed", userID, ms, fmt.Sprintf("%s installation requested", ms.Kind.Label()))

	uid := userIDPtr(userID)
	s.DispatchTask("InstallManagedService", func() (*asynq.Task, error) {
		return jobs.NewInstallTask(ms.ID, uid)
	}, "managed_service_id", ms.ID)

	return dto.ToManagedServiceResponse(ms), nil
}

// Uninstall marks the row as installing and enqueues an uninstall job.
func (s *Service) Uninstall(ctx context.Context, serverID, teamID, userID string, kind types.Kind, removeData bool) error {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return err
	}
	ms, err := s.requireService(ctx, serverID, kind)
	if err != nil {
		return err
	}

	if err := s.repos.ManagedService().Update(ctx, ms.ID, map[string]any{
		"status": types.StatusInstalling, // reuse 'installing' for in-progress lifecycle work
	}); err != nil {
		return fmt.Errorf("failed to mark managed service uninstalling: %w", err)
	}

	activity.RecordEvent(ctx, "uninstalling", userID, ms, fmt.Sprintf("%s uninstall requested", ms.Kind.Label()))

	uid := userIDPtr(userID)
	s.DispatchTask("UninstallManagedService", func() (*asynq.Task, error) {
		return jobs.NewUninstallTask(ms.ID, removeData, uid)
	}, "managed_service_id", ms.ID)

	return nil
}

// Start runs `docker start` against the managed service container.
func (s *Service) Start(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, mstasks.LifecycleStart, types.StatusRunning, "started")
}

// Stop runs `docker stop` against the managed service container.
func (s *Service) Stop(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, mstasks.LifecycleStop, types.StatusStopped, "stopped")
}

// Restart runs `docker restart` against the managed service container.
func (s *Service) Restart(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, mstasks.LifecycleRestart, types.StatusRunning, "restarted")
}

// Logs runs `docker logs --tail N` and returns the captured output.
func (s *Service) Logs(ctx context.Context, serverID, teamID string, kind types.Kind, tail int) (dto.ManagedServiceLogsResponse, error) {
	server, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.ManagedServiceLogsResponse{}, err
	}
	ms, err := s.requireService(ctx, serverID, kind)
	if err != nil {
		return dto.ManagedServiceLogsResponse{}, err
	}
	if ms.IsBusy() {
		return dto.ManagedServiceLogsResponse{}, ErrBusy
	}

	task := mstasks.Logs(mstasks.LogsOptions{
		Kind:      ms.Kind,
		Container: ms.Container,
		Tail:      tail,
	})

	result, err := s.runnerDeps.RunTask(ctx, server, task, true)
	if err != nil {
		return dto.ManagedServiceLogsResponse{}, fmt.Errorf("failed to run logs task: %w", err)
	}

	return dto.ManagedServiceLogsResponse{
		Kind:    ms.Kind,
		Tail:    tail,
		Output:  result.GetOutput(),
		Success: result.IsSuccessful(),
	}, nil
}

func (s *Service) runLifecycle(
	ctx context.Context,
	serverID, teamID, userID string,
	kind types.Kind,
	action mstasks.LifecycleAction,
	endStatus types.Status,
	verb string,
) error {
	server, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}
	ms, err := s.requireService(ctx, serverID, kind)
	if err != nil {
		return err
	}
	if ms.IsBusy() {
		return ErrBusy
	}

	task := mstasks.Lifecycle(mstasks.LifecycleOptions{
		Kind:      ms.Kind,
		Container: ms.Container,
		Action:    action,
	})

	result, err := s.runnerDeps.RunTask(ctx, server, task, true)
	if err != nil {
		return fmt.Errorf("failed to %s managed service: %w", action, err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("%s script failed: %s", action, result.GetOutput())
	}

	if err := s.repos.ManagedService().Update(ctx, ms.ID, map[string]any{
		"status":     endStatus,
		"last_error": nil,
	}); err != nil {
		return fmt.Errorf("failed to update managed service status: %w", err)
	}

	activity.RecordEvent(ctx, verb, userID, ms, fmt.Sprintf("%s %s", ms.Kind.Label(), verb))
	_ = server // server lookup already validates team scope
	return nil
}

func (s *Service) requireService(ctx context.Context, serverID string, kind types.Kind) (*models.ManagedService, error) {
	ms, err := s.repos.ManagedService().FindByServerAndKind(ctx, serverID, kind)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		return nil, ErrNotRunning
	}
	return ms, nil
}

func (s *Service) buildManagedService(serverID, teamID string, req *dto.InstallManagedServiceRequest) *models.ManagedService {
	image := req.Image
	if image == "" {
		image = req.Kind.DefaultImage()
	}

	password := req.Password
	if password == "" {
		password = security.GeneratePassword(24)
	}

	ms := &models.ManagedService{
		Kind:      req.Kind,
		Status:    types.StatusPending,
		Image:     image,
		Container: req.Kind.ContainerName(),
		Volume:    req.Kind.VolumeName(),
		Password:  password,
	}
	ms.ServerID = serverID
	ms.TeamID = teamID

	if req.Kind.RequiresDatabaseName() {
		databaseName := req.DatabaseName
		if databaseName == "" {
			databaseName = "appdb"
		}
		ms.DatabaseName = &databaseName

		username := req.Username
		if username == "" {
			username = "launch"
		}
		ms.Username = &username
	}
	return ms
}

func userIDPtr(userID string) *string {
	if userID == "" {
		return nil
	}
	return &userID
}
