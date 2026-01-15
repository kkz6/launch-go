package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
			Error:   "No output from OPcache status command. The PHP command may have failed silently.",
		}, nil
	}

	// Try to extract JSON from the output (in case there's noise like PHP warnings)
	jsonStr := extractJSON(output)
	if jsonStr == "" {
		// Include raw output (truncated) for debugging
		rawOutput := output
		if len(rawOutput) > 200 {
			rawOutput = rawOutput[:200] + "..."
		}
		return &dto.OpcacheStatusResponse{
			Enabled: false,
			Error:   fmt.Sprintf("No valid JSON output found in response. Raw output: %s", rawOutput),
		}, nil
	}

	// Parse JSON output
	var status dto.OpcacheStatusResponse
	if err := json.Unmarshal([]byte(jsonStr), &status); err != nil {
		return &dto.OpcacheStatusResponse{
			Enabled: false,
			Error:   fmt.Sprintf("Failed to parse OPcache status: %v", err),
		}, nil
	}

	return &status, nil
}

// extractJSON extracts a JSON object from a string that may contain other content
func extractJSON(s string) string {
	// First, try to find JSON by looking for balanced braces
	// Start from the first { and find the matching }
	start := -1
	braceCount := 0

	for i, char := range s {
		if char == '{' {
			if start == -1 {
				start = i
			}
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 && start != -1 {
				candidate := s[start : i+1]
				var js map[string]interface{}
				if err := json.Unmarshal([]byte(candidate), &js); err == nil {
					// Check if this looks like our expected response
					if _, hasEnabled := js["enabled"]; hasEnabled {
						return candidate
					}
				}
				// Reset and look for the next JSON object
				start = -1
			}
		}
	}

	// Fallback: try regex for simpler JSON objects
	re := regexp.MustCompile(`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := re.FindAllString(s, -1)

	// Return the last match (usually the most complete JSON)
	for i := len(matches) - 1; i >= 0; i-- {
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(matches[i]), &js); err == nil {
			if _, hasEnabled := js["enabled"]; hasEnabled {
				return matches[i]
			}
		}
	}

	// If no match with "enabled" field, return the first valid JSON
	for _, match := range matches {
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(match), &js); err == nil {
			return match
		}
	}

	return ""
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
