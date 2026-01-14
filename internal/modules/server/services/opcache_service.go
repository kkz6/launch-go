package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
)

// GetOpcacheStatus returns the OPcache status for a PHP version
func (s *Service) GetOpcacheStatus(ctx context.Context, serverID, teamID, phpID string) (*dto.OpcacheStatusResponse, error) {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	// Find the PHP service
	service, err := s.repo.FindServiceByID(ctx, phpID)
	if err != nil {
		return nil, err
	}

	// Verify it belongs to this server and is a PHP service
	if service.ServerID != serverID {
		return nil, ErrServiceNotFound
	}

	if service.Type != enums.ServiceTypePhp {
		return nil, fmt.Errorf("service is not a PHP installation")
	}

	// Get the PHP version from the software
	software := enums.Software(service.Software)
	version := software.GetVersion()

	// Create and run the task
	task := tasks.GetOpcacheStatus(version)
	runner := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger)

	result, err := runner.Run(ctx)
	if err != nil {
		return &dto.OpcacheStatusResponse{
			Enabled: false,
			Error:   fmt.Sprintf("Failed to get OPcache status: %v", err),
		}, nil
	}

	output := result.GetOutput()
	if output == "" {
		return &dto.OpcacheStatusResponse{
			Enabled: false,
			Error:   "No output from OPcache status command",
		}, nil
	}

	// Parse JSON output
	var status dto.OpcacheStatusResponse
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return &dto.OpcacheStatusResponse{
			Enabled: false,
			Error:   fmt.Sprintf("Failed to parse OPcache status: %v", err),
		}, nil
	}

	return &status, nil
}

// ResetOpcache resets the OPcache for a PHP version
func (s *Service) ResetOpcache(ctx context.Context, serverID, teamID, phpID string) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	// Find the PHP service
	service, err := s.repo.FindServiceByID(ctx, phpID)
	if err != nil {
		return err
	}

	// Verify it belongs to this server and is a PHP service
	if service.ServerID != serverID {
		return ErrServiceNotFound
	}

	if service.Type != enums.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP installation")
	}

	// Get the PHP version from the software
	software := enums.Software(service.Software)
	version := software.GetVersion()

	// Create and run the task asynchronously
	task := tasks.ResetOpcache(version, "")
	runner := tasks.NewTaskRunner(server, task).
		WithDispatcher(s.dispatcher).
		WithLogger(s.Logger).
		AsRoot()

	_, err = runner.Run(ctx)
	return err
}

// ConfigureOpcache configures OPcache settings for a PHP version
func (s *Service) ConfigureOpcache(ctx context.Context, serverID, teamID, phpID string, req *dto.ConfigureOpcacheRequest) error {
	server, err := s.repo.FindServerByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	// Find the PHP service
	service, err := s.repo.FindServiceByID(ctx, phpID)
	if err != nil {
		return err
	}

	// Verify it belongs to this server and is a PHP service
	if service.ServerID != serverID {
		return ErrServiceNotFound
	}

	if service.Type != enums.ServiceTypePhp {
		return fmt.Errorf("service is not a PHP installation")
	}

	// Build settings map
	settings := map[string]string{
		"enable":                  boolToString(req.Enabled),
		"enable_cli":              boolToString(req.EnableCLI),
		"memory_consumption":      strconv.Itoa(req.MemoryConsumption),
		"interned_strings_buffer": strconv.Itoa(req.InternedStringsBuffer),
		"max_accelerated_files":   strconv.Itoa(req.MaxAcceleratedFiles),
		"validate_timestamps":     boolToString(req.ValidateTimestamps),
		"revalidate_freq":         strconv.Itoa(req.RevalidateFreq),
		"save_comments":           boolToString(req.SaveComments),
	}

	// Add JIT settings for PHP 8.0+
	software := enums.Software(service.Software)
	version := software.GetVersion()
	if isPhp8OrNewer(version) && req.JITEnabled {
		settings["jit_buffer_size"] = req.JITBufferSize
		settings["jit"] = req.JITMode
	}

	// Dispatch the configuration job
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewConfigureOpcacheTask(server.ID, service.ID, settings)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func isPhp8OrNewer(version string) bool {
	major, err := strconv.ParseFloat(version, 64)
	if err != nil {
		return false
	}
	return major >= 8.0
}
