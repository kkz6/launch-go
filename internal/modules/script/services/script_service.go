package services

import (
	"context"
	"fmt"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/script/dto"
	"github.com/kkz6/launch-go/internal/modules/script/jobs"
	"github.com/kkz6/launch-go/internal/modules/script/models"
	"github.com/kkz6/launch-go/internal/modules/script/repositories"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/queue"
)

// ScriptService handles business logic for scripts
type ScriptService struct {
	repos       *repositories.Registry
	serverRepos *serverrepos.Registry
	queue       *queue.Client
	logger      *zerolog.Logger
}

// NewScriptService creates a new script service
func NewScriptService(
	repos *repositories.Registry,
	serverRepos *serverrepos.Registry,
	queue *queue.Client,
	logger *zerolog.Logger,
) *ScriptService {
	return &ScriptService{
		repos:       repos,
		serverRepos: serverRepos,
		queue:       queue,
		logger:      logger,
	}
}

// List returns all scripts accessible to the user
func (s *ScriptService) List(ctx context.Context, userID, teamID string) ([]models.Script, error) {
	return s.repos.Script().FindByUserOrTeam(ctx, userID, teamID)
}

// Get returns a script by ID if user has access
func (s *ScriptService) Get(ctx context.Context, scriptID, userID, teamID string) (*models.Script, error) {
	script, err := s.repos.Script().FindByID(ctx, scriptID)
	if err != nil {
		return nil, err
	}

	// Check access
	if script.UserID != userID && (script.TeamID == nil || *script.TeamID != teamID) {
		return nil, repositories.ErrScriptNotFound
	}

	return script, nil
}

// Create creates a new script
func (s *ScriptService) Create(ctx context.Context, userID string, req *dto.CreateScriptRequest) (*models.Script, error) {
	script := &models.Script{
		UserID:  userID,
		TeamID:  req.TeamID,
		Name:    req.Name,
		RunAs:   req.RunAs,
		Content: req.Content,
	}

	if err := s.repos.Script().Create(ctx, script); err != nil {
		return nil, err
	}

	return script, nil
}

// Update updates a script
func (s *ScriptService) Update(ctx context.Context, scriptID, userID, teamID string, req *dto.UpdateScriptRequest) (*models.Script, error) {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Only owner can update
	if script.UserID != userID {
		return nil, fmt.Errorf("only the script owner can update it")
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}

	if req.RunAs != nil {
		updates["user"] = string(*req.RunAs)
	}

	if req.Content != nil {
		updates["content"] = *req.Content
	}

	if req.TeamID != nil {
		updates["team_id"] = req.TeamID
	}

	if len(updates) > 0 {
		if err := s.repos.Script().UpdateFields(ctx, scriptID, updates); err != nil {
			return nil, err
		}
	}

	return s.repos.Script().FindByID(ctx, scriptID)
}

// Delete deletes a script
func (s *ScriptService) Delete(ctx context.Context, scriptID, userID, teamID string) error {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return err
	}

	// Only owner can delete
	if script.UserID != userID {
		return fmt.Errorf("only the script owner can delete it")
	}

	return s.repos.Script().Delete(ctx, scriptID)
}

// Execute executes a script on multiple servers
func (s *ScriptService) Execute(ctx context.Context, scriptID, userID, teamID string, req *dto.ExecuteScriptRequest) (*dto.ExecuteScriptResponse, error) {
	script, err := s.Get(ctx, scriptID, userID, teamID)
	if err != nil {
		return nil, err
	}

	// Validate servers exist and user has access
	for _, serverID := range req.ServerIDs {
		server, err := s.serverRepos.Server().FindByID(ctx, serverID)
		if err != nil {
			return nil, fmt.Errorf("server %s not found", serverID)
		}

		if server.TeamID != teamID {
			return nil, fmt.Errorf("server %s not accessible", serverID)
		}
	}

	// Generate batch ID
	batchID := ulid.Make().String()

	// Determine run-as user type (default from script, can be overridden per execution)
	runAs := script.RunAs
	if req.RunAs != nil {
		runAs = *req.RunAs
	}

	// Create execution records and dispatch jobs
	var executions []*dto.ScriptExecutionResponse

	for _, serverID := range req.ServerIDs {
		execution := &models.ScriptExecution{
			ScriptID: scriptID,
			ServerID: serverID,
			BatchID:  &batchID,
			RunAs:    &runAs,
			Status:   models.ExecutionStatusPending,
		}

		if err := s.repos.Execution().Create(ctx, execution); err != nil {
			return nil, fmt.Errorf("failed to create execution record: %w", err)
		}

		// Dispatch job
		task, err := jobs.NewExecuteScriptTask(execution.ID, scriptID, serverID, teamID)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to create execute script task")
			continue
		}

		if s.queue != nil {
			if _, err := s.queue.Enqueue(task); err != nil {
				s.logger.Error().Err(err).Msg("Failed to enqueue execute script job")
			}
		}

		executions = append(executions, dto.ToExecutionResponse(execution))
	}

	return &dto.ExecuteScriptResponse{
		BatchID:    batchID,
		Executions: executions,
	}, nil
}

// GetExecution returns a single execution
func (s *ScriptService) GetExecution(ctx context.Context, executionID uint64) (*models.ScriptExecution, error) {
	return s.repos.Execution().FindByID(ctx, executionID)
}

// ListExecutions returns all executions for a script
func (s *ScriptService) ListExecutions(ctx context.Context, scriptID, userID, teamID string) ([]models.ScriptExecution, error) {
	// Verify access to script
	if _, err := s.Get(ctx, scriptID, userID, teamID); err != nil {
		return nil, err
	}

	return s.repos.Execution().FindByScript(ctx, scriptID)
}

// GetBatchExecutions returns all executions in a batch
func (s *ScriptService) GetBatchExecutions(ctx context.Context, batchID string) ([]models.ScriptExecution, error) {
	return s.repos.Execution().FindByBatch(ctx, batchID)
}
