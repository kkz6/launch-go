package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
)

// CommandService handles business logic for command execution
type CommandService struct {
	*BaseService
}

// NewCommandService creates a new command service
func NewCommandService(deps *ServiceDeps) *CommandService {
	return &CommandService{
		BaseService: NewBaseService(deps),
	}
}

// Create creates and executes a command
func (s *CommandService) Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateCommandRequest) (*models.Command, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.IsInstalled() {
		return nil, ErrSiteNotInstalled
	}

	cmd := &models.Command{
		Command: req.Command,
		Status:  enums.CommandStatusPending,
	}
	cmd.SiteID = site.ID
	cmd.TeamID = site.TeamID
	cmd.UserID = userID

	if err := s.Repos().Command().Create(ctx, cmd); err != nil {
		return nil, err
	}

	// Dispatch command execution job
	task, err := jobs.NewRunCommandTask(site.ID, cmd.ID)
	if err != nil {
		s.LogError(err, "Failed to create run command task", "command_id", cmd.ID)
	} else if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue run command task", "command_id", cmd.ID)
	}

	s.LogInfo("Command created", "site_id", site.ID, "command_id", cmd.ID)

	return cmd, nil
}

// List returns all commands for a site
func (s *CommandService) List(ctx context.Context, siteID, serverID string) ([]models.Command, error) {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Command().FindBySite(ctx, siteID)
}

// Delete deletes a command by ID
func (s *CommandService) Delete(ctx context.Context, siteID, serverID, commandID string) error {
	// Verify site exists and belongs to server
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	// Find the command
	cmd, err := s.Repos().Command().FindByID(ctx, commandID)
	if err != nil {
		return err
	}

	// Verify command belongs to the site
	if cmd.SiteID != siteID {
		return repositories.ErrCommandNotFound
	}

	// Delete the command
	if err := s.Repos().Command().Delete(ctx, commandID); err != nil {
		return err
	}

	s.LogInfo("Command deleted", "site_id", siteID, "command_id", commandID)

	return nil
}
