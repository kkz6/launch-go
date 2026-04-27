package services

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// CommandService handles business logic for command execution.
type CommandService struct {
	*BaseService
}

// NewCommandService creates a new command service.
func NewCommandService(deps *ServiceDeps) *CommandService {
	return &CommandService{BaseService: NewBaseService(deps)}
}

// Create creates and executes a command. Signature matches CreateDoubleNestedFunc.
func (s *CommandService) Create(ctx context.Context, siteID, serverID, teamID, userID string, req *dto.CreateCommandRequest) (dto.CommandResponse, error) {
	_ = teamID
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return dto.CommandResponse{}, err
	}

	if !site.IsInstalled() {
		return dto.CommandResponse{}, ErrSiteNotInstalled
	}

	cmd := &models.Command{
		Command: req.Command,
		Status:  sitetypes.CommandStatusPending,
	}
	cmd.SiteID = site.ID
	cmd.TeamID = site.TeamID
	cmd.UserID = userID

	if err := s.Repos().Command().Create(ctx, cmd); err != nil {
		return dto.CommandResponse{}, err
	}

	s.DispatchTask("RunCommand", func() (*asynq.Task, error) {
		return jobs.NewRunCommandTask(site.ID, cmd.ID)
	}, "site_id", site.ID, "command_id", cmd.ID)

	return dto.ToCommandResponse(cmd), nil
}

// List returns all commands for a site. Signature matches IndexDoubleNestedFunc.
func (s *CommandService) List(ctx context.Context, siteID, serverID, teamID string) ([]dto.CommandResponse, error) {
	_ = teamID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}
	cmds, err := s.Repos().Command().FindBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CommandResponse, len(cmds))
	for i := range cmds {
		out[i] = dto.ToCommandResponse(&cmds[i])
	}
	return out, nil
}

// Delete deletes a command by ID. Signature matches DeleteDoubleNestedFunc.
func (s *CommandService) Delete(ctx context.Context, commandID, siteID, serverID, teamID, userID string) error {
	_ = teamID
	_ = userID
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	cmd, err := s.Repos().Command().FindByID(ctx, commandID)
	if err != nil {
		return err
	}

	if cmd.SiteID != siteID {
		return fiberutil.NotFound()
	}

	if err := s.Repos().Command().Delete(ctx, commandID); err != nil {
		return err
	}

	s.LogInfo("Command deleted", "site_id", siteID, "command_id", commandID)
	return nil
}
