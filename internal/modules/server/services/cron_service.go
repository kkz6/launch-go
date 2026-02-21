package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const (
	defaultCronUser = "root"
)

// ListCrons returns all cron jobs for a server
func (s *Service) ListCrons(ctx context.Context, serverID, teamID string) ([]models.Cron, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Cron().FindVisibleByServer(ctx, serverID)
}

// CreateCron creates a new cron job
func (s *Service) CreateCron(ctx context.Context, serverID, teamID string, req *dto.CreateCronRequest) (*models.Cron, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	user := defaultCronUser
	if req.User != "" {
		user = req.User
	}

	frequency := ""
	if req.Frequency != nil {
		frequency = *req.Frequency
	}

	cron := &models.Cron{
		SiteID:     req.SiteID,
		User:       user,
		Expression: req.Expression,
		Command:    dbtype.EncryptedString(req.Command),
		Frequency:  frequency,
		Hidden:     false,
	}
	cron.ServerID = serverID

	if err := s.repos.Cron().Create(ctx, cron); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "server", "created", "", cron, "Cron job was created")

	// Broadcast cron created event
	s.BroadcastToTeam(server.TeamID, "cron.created", map[string]interface{}{
		"id":        cron.ID,
		"server_id": serverID,
	})

	if err := s.dispatchCronInstallJob(server, cron); err != nil {
		s.LogError(err, "Failed to dispatch cron install job", "server_id", serverID, "cron_id", cron.ID)
	}

	return cron, nil
}

// UpdateCron updates a cron job
func (s *Service) UpdateCron(ctx context.Context, serverID, teamID, cronID string, req *dto.UpdateCronRequest) (*models.Cron, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}

	cron, err := s.repos.Cron().FindByIDAndServer(ctx, cronID, serverID)
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
		cron.Command = dbtype.EncryptedString(*req.Command)
	}

	if req.Frequency != nil {
		cron.Frequency = *req.Frequency
	}

	if err := s.repos.Cron().Update(ctx, cron); err != nil {
		return nil, err
	}

	activity.RecordWithLog(ctx, "server", "updated", "", cron, "Cron job was updated")

	// Broadcast cron updated event
	s.BroadcastToTeam(teamID, "cron.updated", map[string]interface{}{
		"id":        cron.ID,
		"server_id": serverID,
	})

	return cron, nil
}

// DeleteCron deletes a cron job
func (s *Service) DeleteCron(ctx context.Context, serverID, teamID, cronID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	cron, err := s.repos.Cron().FindByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return err
	}

	activity.RecordWithLog(ctx, "server", "deleted", "", cron, "Cron job deletion requested")

	if err := s.UninstallOrDelete(ctx, cron.IsInstalled(), cronID,
		s.repos.Cron().MarkAsUninstalling,
		s.repos.Cron().Delete,
		func() error { return s.dispatchCronUninstallJob(server, cron) },
	); err != nil {
		return err
	}

	s.BroadcastToTeam(teamID, "cron.deleted", map[string]interface{}{
		"id":        cronID,
		"server_id": serverID,
	})

	return nil
}

func (s *Service) dispatchCronInstallJob(server *models.Server, cron *models.Cron) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewInstallCronTask(server.ID, cron.ID, nil)
	})
}

func (s *Service) dispatchCronUninstallJob(server *models.Server, cron *models.Cron) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewUninstallCronTask(server.ID, cron.ID, nil)
	})
}

// CountCronsBySite counts cron jobs associated with a site
func (s *Service) CountCronsBySite(ctx context.Context, siteID string) (int64, error) {
	return s.repos.Cron().CountBySite(ctx, siteID)
}
