package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListDaemons returns all daemons for a server
func (s *Service) ListDaemons(ctx context.Context, serverID, teamID string) ([]models.Daemon, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindDaemonsByServer(ctx, serverID)
}

// CreateDaemon creates a new daemon
func (s *Service) CreateDaemon(ctx context.Context, serverID, teamID string, req *dto.CreateDaemonRequest) (*models.Daemon, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	user := "root"
	if req.User != "" {
		user = req.User
	}

	processes := 1
	if req.Processes > 0 {
		processes = req.Processes
	}

	stopWaitSeconds := 10
	if req.StopWaitSeconds > 0 {
		stopWaitSeconds = req.StopWaitSeconds
	}

	stopSignal := "SIGTERM"
	if req.StopSignal != nil {
		stopSignal = *req.StopSignal
	}

	daemon := &models.Daemon{
		ServerID:        serverID,
		User:            user,
		Directory:       req.Directory,
		Command:         req.Command,
		Processes:       processes,
		StopWaitSeconds: stopWaitSeconds,
		StopSignal:      stopSignal,
	}

	if err := s.repo.CreateDaemon(ctx, daemon); err != nil {
		return nil, err
	}

	if server.IsProvisioned() {
		if err := s.dispatchDaemonInstallJob(server, daemon); err != nil {
			s.LogError(err, "Failed to dispatch daemon install job", "server_id", serverID, "daemon_id", daemon.ID)
		}
	}

	return daemon, nil
}

// UpdateDaemon updates a daemon
func (s *Service) UpdateDaemon(ctx context.Context, serverID, teamID, daemonID string, req *dto.UpdateDaemonRequest) (*models.Daemon, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	daemon, err := s.repo.FindDaemonByIDAndServer(ctx, daemonID, serverID)
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

	if err := s.repo.UpdateDaemon(ctx, daemon); err != nil {
		return nil, err
	}

	return daemon, nil
}

// DeleteDaemon deletes a daemon
func (s *Service) DeleteDaemon(ctx context.Context, serverID, teamID, daemonID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	daemon, err := s.repo.FindDaemonByIDAndServer(ctx, daemonID, serverID)
	if err != nil {
		return err
	}

	if daemon.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		daemon.UninstallationRequestedAt = &now
		if err := s.repo.UpdateDaemon(ctx, daemon); err != nil {
			return err
		}

		if err := s.dispatchDaemonUninstallJob(server, daemon); err != nil {
			s.LogError(err, "Failed to dispatch daemon uninstall job", "server_id", serverID, "daemon_id", daemonID)
		}

		return nil
	}

	return s.repo.DeleteDaemon(ctx, daemonID)
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
