package services

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// ListCrons returns all cron jobs for a server
func (s *Service) ListCrons(ctx context.Context, serverID, teamID string) ([]models.Cron, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repo.FindVisibleCronsByServer(ctx, serverID)
}

// CreateCron creates a new cron job
func (s *Service) CreateCron(ctx context.Context, serverID, teamID string, req *dto.CreateCronRequest) (*models.Cron, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	user := "root"
	if req.User != "" {
		user = req.User
	}

	frequency := ""
	if req.Frequency != nil {
		frequency = *req.Frequency
	}

	cron := &models.Cron{
		ServerID:   serverID,
		SiteID:     req.SiteID,
		User:       user,
		Expression: req.Expression,
		Command:    req.Command,
		Frequency:  frequency,
		Hidden:     false,
	}

	if err := s.repo.CreateCron(ctx, cron); err != nil {
		return nil, err
	}

	if server.IsProvisioned() {
		if err := s.dispatchCronInstallJob(server, cron); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("cron_id", cron.ID).
				Msg("Failed to dispatch cron install job")
		}
	}

	return cron, nil
}

// UpdateCron updates a cron job
func (s *Service) UpdateCron(ctx context.Context, serverID, teamID, cronID string, req *dto.UpdateCronRequest) (*models.Cron, error) {
	if _, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	cron, err := s.repo.FindCronByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return nil, err
	}

	if req.User != nil {
		cron.User = *req.User
	}

	if req.Expression != nil {
		cron.Expression = *req.Expression
	}

	if req.Command != nil {
		cron.Command = *req.Command
	}

	if req.Frequency != nil {
		cron.Frequency = *req.Frequency
	}

	if err := s.repo.UpdateCron(ctx, cron); err != nil {
		return nil, err
	}

	return cron, nil
}

// DeleteCron deletes a cron job
func (s *Service) DeleteCron(ctx context.Context, serverID, teamID, cronID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	cron, err := s.repo.FindCronByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return err
	}

	if cron.IsInstalled() && server.IsProvisioned() {
		now := time.Now()
		cron.UninstallationRequestedAt = &now
		if err := s.repo.UpdateCron(ctx, cron); err != nil {
			return err
		}

		if err := s.dispatchCronUninstallJob(server, cron); err != nil {
			s.logger.Error().Err(err).
				Str("server_id", serverID).
				Str("cron_id", cronID).
				Msg("Failed to dispatch cron uninstall job")
		}

		return nil
	}

	return s.repo.DeleteCron(ctx, cronID)
}

func (s *Service) dispatchCronInstallJob(server *models.Server, cron *models.Cron) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCronTask(server.ID, cron.ID, "install")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}

func (s *Service) dispatchCronUninstallJob(server *models.Server, cron *models.Cron) error {
	if s.queue == nil {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCronTask(server.ID, cron.ID, "uninstall")
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueDefault(task)

	return err
}
