package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

var (
	ErrComposerNotInstalled = fiberutil.BadRequest("Composer package manager is not installed on this server")
)

// GetComposerAuth retrieves the Composer auth.json configuration from the server
func (s *Service) GetComposerAuth(ctx context.Context, serverID, teamID string) (*dto.ComposerAuthResponse, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Check if Composer is installed
	composerService, err := s.repos.Service().FindOneByServerAndType(ctx, serverID, enums.ServiceTypeComposer)
	if err != nil && !fiberutil.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check composer installation: %w", err)
	}
	if composerService == nil {
		return nil, ErrComposerNotInstalled
	}

	// Get the username for the path
	username := server.GetUsername()
	composerAuthPath := fmt.Sprintf("/home/%s/.config/composer/auth.json", username)

	// Create and run the task to get the file contents
	task := tasks.GetFile(tasks.GetFileConfig{Path: composerAuthPath})
	runner := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger)

	result, err := runner.Run(ctx)
	if err != nil {
		// File might not exist yet, return empty config
		return &dto.ComposerAuthResponse{
			Path:     composerAuthPath,
			Contents: map[string]interface{}{},
		}, nil
	}

	output := result.GetOutput()

	// If file doesn't exist or is empty, return empty config
	if output == "" || !result.IsSuccessful() {
		return &dto.ComposerAuthResponse{
			Path:     composerAuthPath,
			Contents: map[string]interface{}{},
		}, nil
	}

	// Parse JSON output
	var contents interface{}
	if err := json.Unmarshal([]byte(output), &contents); err != nil {
		// If parsing fails, return raw content as string
		return &dto.ComposerAuthResponse{
			Path:     composerAuthPath,
			Contents: output,
		}, nil
	}

	return &dto.ComposerAuthResponse{
		Path:     composerAuthPath,
		Contents: contents,
	}, nil
}

// UpdateComposerAuth updates the Composer auth.json configuration on the server
func (s *Service) UpdateComposerAuth(ctx context.Context, serverID, teamID string, req *dto.UpdateComposerAuthRequest) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	// Check if Composer is installed
	composerService, err := s.repos.Service().FindOneByServerAndType(ctx, serverID, enums.ServiceTypeComposer)
	if err != nil && !fiberutil.IsNotFound(err) {
		return fmt.Errorf("failed to check composer installation: %w", err)
	}
	if composerService == nil {
		return ErrComposerNotInstalled
	}

	// Validate JSON
	var jsonContent interface{}
	if err := json.Unmarshal([]byte(req.Contents), &jsonContent); err != nil {
		return fiberutil.BadRequest("Invalid JSON content")
	}

	// Get the username for the path
	username := server.GetUsername()
	composerAuthPath := fmt.Sprintf("/home/%s/.config/composer/auth.json", username)

	// Create and run the task to upload the file
	task := tasks.UploadFile(tasks.UploadFileConfig{
		Path:     composerAuthPath,
		Contents: req.Contents,
		Mode:     "644",
	})
	runner := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger).
		AsRoot()

	result, err := runner.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to update Composer auth: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to update Composer auth: %s", result.GetOutput())
	}

	return nil
}
