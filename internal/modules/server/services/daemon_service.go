package services

import (
	"context"
	"time"

	"gorm.io/gorm"

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

// ListDaemons returns all daemons for a server
func (s *Service) ListDaemons(ctx context.Context, serverID, teamID string) ([]models.Daemon, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Daemon().FindByServer(ctx, serverID)
}

// CreateDaemon creates a new daemon
func (s *Service) CreateDaemon(ctx context.Context, serverID, teamID string, req *dto.CreateDaemonRequest) (*models.Daemon, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	activity.RecordWithLog(ctx, "server", "created", "", daemon, "Daemon was created")

	if server.IsProvisioned() {
		if err := s.dispatchDaemonInstallJob(server, daemon); err != nil {
			s.LogError(err, "Failed to dispatch daemon install job", "server_id", serverID, "daemon_id", daemon.ID)
		}
	}

	return daemon, nil
}

// UpdateDaemon updates a daemon
func (s *Service) UpdateDaemon(ctx context.Context, serverID, teamID, daemonID string, req *dto.UpdateDaemonRequest) (*models.Daemon, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	activity.RecordWithLog(ctx, "server", "updated", "", daemon, "Daemon was updated")

	return daemon, nil
}

// DeleteDaemon deletes a daemon
func (s *Service) DeleteDaemon(ctx context.Context, serverID, teamID, daemonID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "server", "deleted", "", daemon, "Daemon deletion requested")

	if daemon.IsInstalled() && server.IsProvisioned() {
		return s.WithTransaction(ctx, func(tx *gorm.DB) error {
			now := time.Now()
			daemon.UninstallationRequestedAt = &now
			if err := tx.Save(daemon).Error; err != nil {
				return err
			}

			return s.dispatchDaemonUninstallJob(server, daemon)
		})
	}

	return s.repos.Daemon().Delete(ctx, daemonID)
}

func (s *Service) dispatchDaemonInstallJob(server *models.Server, daemon *models.Daemon) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewInstallDaemonTask(server.ID, daemon.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func (s *Service) dispatchDaemonUninstallJob(server *models.Server, daemon *models.Daemon) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewUninstallDaemonTask(server.ID, daemon.ID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

// RestartDaemon restarts a daemon on the server
func (s *Service) RestartDaemon(ctx context.Context, serverID, teamID, daemonID string, userID *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repos.Daemon().FindByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	if !daemon.IsInstalled() || !server.IsProvisioned() {
		return ErrDaemonNotInstalled
	}

	activity.RecordWithLog(ctx, "server", "restarting", "", daemon, "Daemon restart requested")

	task, err := jobs.NewRestartDaemonTask(server.ID, daemon.ID, userID)
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

// SyncDaemonsStatus triggers a status synchronization for all daemons on a server
func (s *Service) SyncDaemonsStatus(ctx context.Context, serverID, teamID string, userID *string) error {
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

	// Batch update last status check time for all daemons on this server
	now := time.Now()
	if err := s.repos.Daemon().UpdateLastStatusCheckByServer(ctx, serverID, now); err != nil {
		s.LogError(err, "Failed to batch update daemon last status check", "server_id", serverID)
		return err
	}

	// Dispatch the sync job to check supervisor status on the server
	task, err := jobs.NewSyncDaemonsTask(server.ID, userID)
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
