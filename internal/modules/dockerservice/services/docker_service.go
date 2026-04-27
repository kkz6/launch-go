package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/dto"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/jobs"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/models"
	dstasks "github.com/kkz6/launch-go/internal/modules/dockerservice/tasks"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ListByServer returns every docker service for a server.
func (s *Service) ListByServer(ctx context.Context, serverID, teamID string) ([]dto.DockerServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	services, err := s.repos.DockerService().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return dto.ToDockerServiceResponseList(services), nil
}

// Get returns a single docker service for a server+kind.
func (s *Service) Get(ctx context.Context, serverID, teamID string, kind types.Kind) (dto.DockerServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.DockerServiceResponse{}, err
	}
	ms, err := s.repos.DockerService().FindByServerAndKind(ctx, serverID, kind)
	if err != nil {
		return dto.DockerServiceResponse{}, err
	}
	if ms == nil {
		return dto.DockerServiceResponse{}, nil
	}
	return dto.ToDockerServiceResponse(ms), nil
}

// Install creates the row and dispatches the install job.
func (s *Service) Install(ctx context.Context, serverID, teamID, userID string, req *dto.InstallDockerServiceRequest) (dto.DockerServiceResponse, error) {
	if _, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.DockerServiceResponse{}, err
	}

	if !req.Kind.IsValid() {
		return dto.DockerServiceResponse{}, fmt.Errorf("invalid kind: %s", req.Kind)
	}

	existing, err := s.repos.DockerService().FindByServerAndKind(ctx, serverID, req.Kind)
	if err != nil {
		return dto.DockerServiceResponse{}, err
	}
	if existing != nil {
		return dto.DockerServiceResponse{}, ErrAlreadyInstalled
	}

	ms := s.buildDockerService(serverID, teamID, req)

	if err := s.repos.DockerService().Create(ctx, ms); err != nil {
		return dto.DockerServiceResponse{}, fmt.Errorf("failed to create docker service: %w", err)
	}

	activity.RecordEvent(ctx, "installed", userID, ms, fmt.Sprintf("%s installation requested", ms.Kind.Label()))

	uid := userIDPtr(userID)
	s.DispatchTask("InstallDockerService", func() (*asynq.Task, error) {
		return jobs.NewInstallTask(ms.ID, uid)
	}, "docker_service_id", ms.ID)

	return dto.ToDockerServiceResponse(ms), nil
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

	if err := s.repos.DockerService().Update(ctx, ms.ID, map[string]any{
		"status": types.StatusInstalling, // reuse 'installing' for in-progress lifecycle work
	}); err != nil {
		return fmt.Errorf("failed to mark docker service uninstalling: %w", err)
	}

	activity.RecordEvent(ctx, "uninstalling", userID, ms, fmt.Sprintf("%s uninstall requested", ms.Kind.Label()))

	uid := userIDPtr(userID)
	s.DispatchTask("UninstallDockerService", func() (*asynq.Task, error) {
		return jobs.NewUninstallTask(ms.ID, removeData, uid)
	}, "docker_service_id", ms.ID)

	return nil
}

// Start runs `docker start` against the docker service container.
func (s *Service) Start(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, dstasks.LifecycleStart, types.StatusRunning, "started")
}

// Stop runs `docker stop` against the docker service container.
func (s *Service) Stop(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, dstasks.LifecycleStop, types.StatusStopped, "stopped")
}

// Restart runs `docker restart` against the docker service container.
func (s *Service) Restart(ctx context.Context, serverID, teamID, userID string, kind types.Kind) error {
	return s.runLifecycle(ctx, serverID, teamID, userID, kind, dstasks.LifecycleRestart, types.StatusRunning, "restarted")
}

// Logs runs `docker logs --tail N` and returns the captured output.
func (s *Service) Logs(ctx context.Context, serverID, teamID string, kind types.Kind, tail int) (dto.DockerServiceLogsResponse, error) {
	server, err := s.serverReader.FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.DockerServiceLogsResponse{}, err
	}
	ms, err := s.requireService(ctx, serverID, kind)
	if err != nil {
		return dto.DockerServiceLogsResponse{}, err
	}
	if ms.IsBusy() {
		return dto.DockerServiceLogsResponse{}, ErrBusy
	}

	task := dstasks.Logs(dstasks.LogsOptions{
		Kind:      ms.Kind,
		Container: ms.Container,
		Tail:      tail,
	})

	result, err := s.runnerDeps.RunTask(ctx, server, task, true)
	if err != nil {
		return dto.DockerServiceLogsResponse{}, fmt.Errorf("failed to run logs task: %w", err)
	}

	return dto.DockerServiceLogsResponse{
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
	action dstasks.LifecycleAction,
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

	task := dstasks.Lifecycle(dstasks.LifecycleOptions{
		Kind:      ms.Kind,
		Container: ms.Container,
		Action:    action,
	})

	result, err := s.runnerDeps.RunTask(ctx, server, task, true)
	if err != nil {
		return fmt.Errorf("failed to %s docker service: %w", action, err)
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("%s script failed: %s", action, result.GetOutput())
	}

	if err := s.repos.DockerService().Update(ctx, ms.ID, map[string]any{
		"status":     endStatus,
		"last_error": nil,
	}); err != nil {
		return fmt.Errorf("failed to update docker service status: %w", err)
	}

	activity.RecordEvent(ctx, verb, userID, ms, fmt.Sprintf("%s %s", ms.Kind.Label(), verb))
	_ = server // server lookup already validates team scope
	return nil
}

func (s *Service) requireService(ctx context.Context, serverID string, kind types.Kind) (*models.DockerService, error) {
	ms, err := s.repos.DockerService().FindByServerAndKind(ctx, serverID, kind)
	if err != nil {
		return nil, err
	}
	if ms == nil {
		return nil, ErrNotRunning
	}
	return ms, nil
}

func (s *Service) buildDockerService(serverID, teamID string, req *dto.InstallDockerServiceRequest) *models.DockerService {
	image := req.Image
	if image == "" {
		image = req.Kind.DefaultImage()
	}

	password := req.Password
	if password == "" {
		password = security.GeneratePassword(24)
	}

	ms := &models.DockerService{
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
