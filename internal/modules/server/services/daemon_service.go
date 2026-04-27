package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const (
	defaultDaemonUser       = "root"
	defaultDaemonProcesses  = 1
	defaultDaemonStopWait   = 10
	defaultDaemonStopSignal = "SIGTERM"
)

// ListDaemons returns all daemons for a server. Signature matches IndexNestedFunc.
func (s *Service) ListDaemons(ctx context.Context, serverID, teamID string) ([]dto.DaemonResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	daemons, err := s.repos.Daemon().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.DaemonResponse, len(daemons))
	for i := range daemons {
		out[i] = dto.ToDaemonResponse(&daemons[i])
	}
	return out, nil
}

// CreateDaemon creates a new daemon. Signature matches CreateNestedFunc.
func (s *Service) CreateDaemon(ctx context.Context, serverID, teamID, userID string, req *dto.CreateDaemonRequest) (dto.DaemonResponse, error) {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.DaemonResponse{}, err
	}

	user := defaultDaemonUser
	if req.User != "" {
		user = req.User
	}
	processes := defaultDaemonProcesses
	if req.Processes > 0 {
		processes = req.Processes
	}
	stopWaitSeconds := defaultDaemonStopWait
	if req.StopWaitSeconds > 0 {
		stopWaitSeconds = req.StopWaitSeconds
	}
	stopSignal := defaultDaemonStopSignal
	if req.StopSignal != nil {
		stopSignal = *req.StopSignal
	}

	daemon := &models.Daemon{
		User:            user,
		Directory:       req.Directory,
		Command:         req.Command,
		Processes:       processes,
		StopWaitSeconds: stopWaitSeconds,
		StopSignal:      stopSignal,
	}
	daemon.ServerID = serverID

	if err := s.repos.Daemon().Create(ctx, daemon); err != nil {
		return dto.DaemonResponse{}, err
	}

	activity.RecordWithLog(ctx, "server", "created", "", daemon, "Daemon was created")

	if err := s.dispatchDaemonInstallJob(server, daemon); err != nil {
		s.LogError(err, "Failed to dispatch daemon install job", "server_id", serverID, "daemon_id", daemon.ID)
	}
	return dto.ToDaemonResponse(daemon), nil
}

// UpdateDaemon updates a daemon. Signature matches UpdateNestedFunc.
func (s *Service) UpdateDaemon(ctx context.Context, daemonID, serverID, teamID, userID string, req *dto.UpdateDaemonRequest) (dto.DaemonResponse, error) {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return dto.DaemonResponse{}, err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return dto.DaemonResponse{}, err
	}

	if req.User != nil {
		daemon.User = *req.User
	}
	if req.Directory != nil {
		daemon.Directory = req.Directory
	}
	if req.Command != nil {
		daemon.Command = *req.Command
	}
	if req.Processes != nil {
		daemon.Processes = *req.Processes
	}
	if req.StopWaitSeconds != nil {
		daemon.StopWaitSeconds = *req.StopWaitSeconds
	}
	if req.StopSignal != nil {
		daemon.StopSignal = *req.StopSignal
	}

	if err := s.repos.Daemon().Update(ctx, daemon); err != nil {
		return dto.DaemonResponse{}, err
	}

	activity.RecordWithLog(ctx, "server", "updated", "", daemon, "Daemon was updated")

	if err := s.dispatchDaemonInstallJob(server, daemon); err != nil {
		s.LogError(err, "Failed to dispatch daemon install job", "server_id", serverID, "daemon_id", daemon.ID)
	}
	return dto.ToDaemonResponse(daemon), nil
}

// DeleteDaemon deletes a daemon. Signature matches DeleteNestedFunc.
func (s *Service) DeleteDaemon(ctx context.Context, daemonID, serverID, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "server", "deleted", "", daemon, "Daemon deletion requested")

	return s.UninstallOrDelete(ctx, daemon.IsInstalled(), daemonID,
		s.repos.Daemon().MarkAsUninstalling,
		s.repos.Daemon().Delete,
		func() error { return s.dispatchDaemonUninstallJob(server, daemon) },
	)
}

func (s *Service) dispatchDaemonInstallJob(server *models.Server, daemon *models.Daemon) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewInstallDaemonTask(server.ID, daemon.ID, nil)
	})
}

func (s *Service) dispatchDaemonUninstallJob(server *models.Server, daemon *models.Daemon) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewUninstallDaemonTask(server.ID, daemon.ID, nil)
	})
}

// RestartDaemon restarts a daemon on the server. Signature matches
// ActionItemNestedFunc.
func (s *Service) RestartDaemon(ctx context.Context, daemonID, serverID, teamID, userID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	if !daemon.IsInstalled() {
		return ErrDaemonNotInstalled
	}

	activity.RecordWithLog(ctx, "server", "restarting", "", daemon, "Daemon restart requested")

	uid := userIDPtr(userID)
	task, err := jobs.NewRestartDaemonTask(server.ID, daemon.ID, uid)
	if err != nil {
		s.LogError(err, "Failed to create restart daemon task")
		return err
	}
	if s.HasQueue() {
		if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue restart daemon job")
			return err
		}
	}
	s.LogInfo("Daemon restart initiated", "server_id", serverID, "daemon_id", daemonID)
	return nil
}

// SyncDaemonsStatus triggers a status synchronization for all daemons on
// a server. Signature matches ActionNestedFunc.
func (s *Service) SyncDaemonsStatus(ctx context.Context, serverID, teamID, userID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemons, err := s.repos.Daemon().FindByServer(ctx, serverID)
	if err != nil {
		return err
	}
	if len(daemons) == 0 {
		s.LogInfo("No daemons to sync", "server_id", serverID)
		return nil
	}

	uid := userIDPtr(userID)
	task, err := jobs.NewSyncDaemonsTask(server.ID, uid)
	if err != nil {
		s.LogError(err, "Failed to create sync daemons task")
		return err
	}
	if s.HasQueue() {
		if err := s.EnqueueTask(task); err != nil {
			s.LogError(err, "Failed to enqueue sync daemons job")
			return err
		}
	}
	s.LogInfo("Daemon sync initiated", "server_id", serverID, "daemon_count", len(daemons))
	return nil
}

// userIDPtr returns nil for empty userID, otherwise a pointer. Used at
// the boundary where downstream APIs accept *string.
func userIDPtr(userID string) *string {
	if userID == "" {
		return nil
	}
	return &userID
}
