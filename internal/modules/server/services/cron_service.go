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

const defaultCronUser = "root"

// ListCrons lists all visible cron jobs for a server. Signature matches
// IndexNestedFunc.
func (s *Service) ListCrons(ctx context.Context, serverID, teamID string) ([]dto.CronResponse, error) {
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return nil, err
	}
	crons, err := s.repos.Cron().FindVisibleByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CronResponse, len(crons))
	for i := range crons {
		out[i] = dto.ToCronResponse(&crons[i])
	}
	return out, nil
}

// CreateCron creates a new cron job and returns the response DTO.
// Signature matches CreateNestedFunc.
func (s *Service) CreateCron(ctx context.Context, serverID, teamID, userID string, req *dto.CreateCronRequest) (dto.CronResponse, error) {
	_ = userID
	cron, err := s.CreateCronRaw(ctx, serverID, teamID, req)
	if err != nil {
		return dto.CronResponse{}, err
	}
	return dto.ToCronResponse(cron), nil
}

// CreateCronRaw is the model-returning entrypoint preserved for
// cross-module callers (e.g. site provisioning). HTTP handlers should
// use CreateCron (DTO-returning).
func (s *Service) CreateCronRaw(ctx context.Context, serverID, teamID string, req *dto.CreateCronRequest) (*models.Cron, error) {
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
	s.BroadcastToTeam(server.TeamID, "cron.created", map[string]interface{}{
		"id":        cron.ID,
		"server_id": serverID,
	})

	if err := s.dispatchCronInstallJob(server, cron); err != nil {
		s.LogError(err, "Failed to dispatch cron install job", "server_id", serverID, "cron_id", cron.ID)
	}
	return cron, nil
}

// UpdateCron updates a cron job and returns the response DTO. Signature
// matches UpdateNestedFunc: (ctx, id, parentID, teamID, userID, req).
func (s *Service) UpdateCron(ctx context.Context, cronID, serverID, teamID, userID string, req *dto.UpdateCronRequest) (dto.CronResponse, error) {
	_ = userID
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID); err != nil {
		return dto.CronResponse{}, err
	}

	cron, err := s.repos.Cron().FindByIDAndServer(ctx, cronID, serverID)
	if err != nil {
		return dto.CronResponse{}, err
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
		return dto.CronResponse{}, err
	}

	activity.RecordWithLog(ctx, "server", "updated", "", cron, "Cron job was updated")
	s.BroadcastToTeam(teamID, "cron.updated", map[string]interface{}{
		"id":        cron.ID,
		"server_id": serverID,
	})

	return dto.ToCronResponse(cron), nil
}

// DeleteCron deletes a cron job. Signature matches DeleteNestedFunc:
// (ctx, id, parentID, teamID, userID).
func (s *Service) DeleteCron(ctx context.Context, cronID, serverID, teamID, userID string) error {
	_ = userID
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

// CountCronsBySite counts cron jobs associated with a site.
func (s *Service) CountCronsBySite(ctx context.Context, siteID string) (int64, error) {
	return s.repos.Cron().CountBySite(ctx, siteID)
}
