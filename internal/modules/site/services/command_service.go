package services

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// CommandService handles business logic for command execution
type CommandService struct {
	*BaseService
}

// NewCommandService creates a new command service
func NewCommandService(
	siteRepo *repositories.SiteRepository,
	deploymentRepo *repositories.DeploymentRepository,
	certificateRepo *repositories.CertificateRepository,
	queueRepo *repositories.QueueRepository,
	commandRepo *repositories.CommandRepository,
	redirectRepo *repositories.RedirectRepository,
	releaseRepo *repositories.ReleaseRepository,
	queueClient *queue.Client,
	ws *websocket.Hub,
	logger *zerolog.Logger,
) *CommandService {
	return &CommandService{
		BaseService: NewBaseService(
			siteRepo,
			deploymentRepo,
			certificateRepo,
			queueRepo,
			commandRepo,
			redirectRepo,
			releaseRepo,
			queueClient,
			ws,
			logger,
		),
	}
}

// Create creates and executes a command
func (s *CommandService) Create(ctx context.Context, siteID, serverID, userID string, req *dto.CreateCommandRequest) (*models.Command, error) {
	site, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.IsInstalled() {
		return nil, ErrSiteNotInstalled
	}

	cmd := &models.Command{
		SiteID:  site.ID,
		UserID:  userID,
		Command: req.Command,
		Status:  enums.CommandStatusPending,
	}

	if err := s.commandRepo.Create(ctx, cmd); err != nil {
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
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.commandRepo.FindBySite(ctx, siteID)
}

// Delete deletes a command by ID
func (s *CommandService) Delete(ctx context.Context, siteID, serverID, commandID string) error {
	// Verify site exists and belongs to server
	if _, err := s.siteRepo.FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	// Find the command
	cmd, err := s.commandRepo.FindByID(ctx, commandID)
	if err != nil {
		return err
	}

	// Verify command belongs to the site
	if cmd.SiteID != siteID {
		return repositories.ErrCommandNotFound
	}

	// Delete the command
	if err := s.commandRepo.Delete(ctx, commandID); err != nil {
		return err
	}

	s.LogInfo("Command deleted", "site_id", siteID, "command_id", commandID)

	return nil
}
