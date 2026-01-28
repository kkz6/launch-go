package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/markers"

	"github.com/rs/zerolog"
)

// ErrTaskCompleted signals that the task has completed (not an error)
var ErrTaskCompleted = errors.New("task completed")

// MarkerHandler is called when a marker is detected in the output stream.
// It allows tasks to respond to progress updates, step completions, etc.
type MarkerHandler interface {
	// OnMarker is called for each marker detected in the output.
	// Return an error to stop streaming (e.g., ErrTaskCompleted).
	OnMarker(ctx context.Context, taskID string, marker *markers.Marker) error
}

// MarkerHandlerFunc is a function adapter for MarkerHandler
type MarkerHandlerFunc func(ctx context.Context, taskID string, marker *markers.Marker) error

func (f MarkerHandlerFunc) OnMarker(ctx context.Context, taskID string, marker *markers.Marker) error {
	return f(ctx, taskID, marker)
}

// StreamMonitor monitors task output via persistent SSH connection
type StreamMonitor struct {
	logger            *zerolog.Logger
	wsHub             SimpleBroadcaster
	broadcastInterval time.Duration
	activeStreams     map[string]context.CancelFunc
	markerHandler     MarkerHandler
	mu                sync.RWMutex
}

// StreamMonitorConfig holds configuration for the stream monitor
type StreamMonitorConfig struct {
	BroadcastInterval time.Duration // How often to broadcast accumulated output
	MarkerHandler     MarkerHandler // Optional handler for markers
}

// NewStreamMonitor creates a new stream monitor
func NewStreamMonitor(logger *zerolog.Logger, wsHub SimpleBroadcaster, cfg *StreamMonitorConfig) *StreamMonitor {
	interval := 2 * time.Second
	var markerHandler MarkerHandler

	if cfg != nil {
		if cfg.BroadcastInterval > 0 {
			interval = cfg.BroadcastInterval
		}
		markerHandler = cfg.MarkerHandler
	}

	return &StreamMonitor{
		logger:            logger,
		wsHub:             wsHub,
		broadcastInterval: interval,
		activeStreams:     make(map[string]context.CancelFunc),
		markerHandler:     markerHandler,
	}
}

// SetMarkerHandler sets the marker handler for this stream monitor
func (m *StreamMonitor) SetMarkerHandler(handler MarkerHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markerHandler = handler
}

// StreamResult contains the final result of streaming a task
type StreamResult struct {
	TaskID     string
	Output     string
	ExitCode   int
	Status     string // "finished", "failed", "timeout"
	Error      error
	FinishedAt time.Time
}

// StreamTaskOutput starts streaming output from a remote task
// This maintains a single SSH connection and uses tail -f to stream the log file
func (m *StreamMonitor) StreamTaskOutput(
	ctx context.Context,
	conn *Connection,
	taskID string,
	onUpdate func(output string, status string),
	onComplete func(result *StreamResult),
) error {
	// Check if already streaming this task
	m.mu.Lock()
	if _, exists := m.activeStreams[taskID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("already streaming task %s", taskID)
	}

	// Create cancellable context for this stream
	streamCtx, cancel := context.WithCancel(ctx)
	m.activeStreams[taskID] = cancel
	m.mu.Unlock()

	// Cleanup on exit
	defer func() {
		m.mu.Lock()
		delete(m.activeStreams, taskID)
		m.mu.Unlock()
	}()

	// Create SSH client
	sshClient, err := conn.Dial()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	m.logger.Info().
		Str("task_id", taskID).
		Str("host", conn.Host).
		Msg("Started streaming task output")

	// Build the log file path
	taskPaths := paths.GetTaskPaths(conn.GetScriptPath(), taskID)

	// Wait for log file to exist (task may not have started writing yet)
	waitCmd := fmt.Sprintf("while [ ! -f %s ]; do sleep 0.5; done; echo 'ready'", taskPaths.Output)
	if _, err := sshClient.Run(streamCtx, waitCmd); err != nil {
		if streamCtx.Err() != nil {
			return streamCtx.Err()
		}
		m.logger.Warn().Err(err).Str("task_id", taskID).Msg("Log file not found, continuing anyway")
	}

	// Use tail -f to stream the log file
	// -n +1 starts from the beginning of the file
	tailCmd := fmt.Sprintf("tail -n +1 -f %s 2>/dev/null", taskPaths.Output)

	var outputBuffer strings.Builder
	var lastBroadcast time.Time
	var exitCode int
	var finalStatus string

	err = sshClient.StreamOutput(streamCtx, tailCmd, func(line string) error {
		// Check for markers
		if marker := markers.Parse(line); marker != nil {
			return m.handleMarker(streamCtx, taskID, marker, &exitCode, &finalStatus)
		}

		// Regular output line - add to buffer
		outputBuffer.WriteString(line + "\n")

		// Broadcast accumulated output periodically
		if time.Since(lastBroadcast) >= m.broadcastInterval {
			output := outputBuffer.String()
			if onUpdate != nil {
				onUpdate(output, "running")
			}
			m.broadcastOutput(taskID, output, "running")
			lastBroadcast = time.Now()
		}

		return nil
	})

	// Handle completion
	result := &StreamResult{
		TaskID:     taskID,
		Output:     outputBuffer.String(),
		ExitCode:   exitCode,
		Status:     finalStatus,
		FinishedAt: time.Now(),
	}

	if err != nil && !errors.Is(err, ErrTaskCompleted) {
		result.Error = err
		if finalStatus == "" {
			result.Status = "failed"
		}
	}

	// Final broadcast
	m.broadcastOutput(taskID, result.Output, result.Status)

	if onComplete != nil {
		onComplete(result)
	}

	m.logger.Info().
		Str("task_id", taskID).
		Str("status", result.Status).
		Int("exit_code", result.ExitCode).
		Msg("Task streaming completed")

	return nil
}

// StopStreaming stops streaming for a specific task
func (m *StreamMonitor) StopStreaming(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, exists := m.activeStreams[taskID]; exists {
		cancel()
		delete(m.activeStreams, taskID)
	}
}

// StopAll stops all active streams
func (m *StreamMonitor) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for taskID, cancel := range m.activeStreams {
		cancel()
		delete(m.activeStreams, taskID)
	}
}

// IsStreaming checks if a task is being streamed
func (m *StreamMonitor) IsStreaming(taskID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.activeStreams[taskID]
	return exists
}

// ActiveStreamCount returns the number of active streams
func (m *StreamMonitor) ActiveStreamCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.activeStreams)
}

func (m *StreamMonitor) broadcastOutput(taskID, output, status string) {
	if m.wsHub == nil {
		return
	}

	m.wsHub.Broadcast(broadcast.TaskChannel(taskID), "task.output", map[string]interface{}{
		"task_id": taskID,
		"output":  output,
		"status":  status,
	})
}

func (m *StreamMonitor) broadcastProgress(taskID string, progress int) {
	if m.wsHub == nil {
		return
	}

	m.wsHub.Broadcast(broadcast.TaskChannel(taskID), "task.progress", map[string]interface{}{
		"task_id":  taskID,
		"progress": progress,
	})
}

// handleMarker processes a parsed marker and updates state accordingly
func (m *StreamMonitor) handleMarker(
	ctx context.Context,
	taskID string,
	marker *markers.Marker,
	exitCode *int,
	finalStatus *string,
) error {
	// Call custom marker handler if set
	m.mu.RLock()
	handler := m.markerHandler
	m.mu.RUnlock()

	if handler != nil {
		if err := handler.OnMarker(ctx, taskID, marker); err != nil {
			return err
		}
	}

	// Handle built-in marker types
	switch marker.Type {
	case markers.Progress:
		m.broadcastProgress(taskID, marker.ProgressValue())

	case markers.StepCompleted:
		m.broadcastMarkerEvent(taskID, "task.step_completed", map[string]interface{}{
			"step": marker.Value,
		})

	case markers.SoftwareInstalled:
		m.broadcastMarkerEvent(taskID, "task.software_installed", map[string]interface{}{
			"software": marker.Value,
		})

	case markers.Status:
		m.broadcastMarkerEvent(taskID, "task.status", map[string]interface{}{
			"message": marker.Value,
		})

	case markers.Error:
		m.broadcastMarkerEvent(taskID, "task.error", map[string]interface{}{
			"message": marker.Value,
		})

	case markers.ExitCode:
		code := marker.ExitCodeValue()
		*exitCode = code
		if code == 0 {
			*finalStatus = "finished"
		} else {
			*finalStatus = "failed"
		}
		return ErrTaskCompleted
	}

	return nil
}

// broadcastMarkerEvent broadcasts a marker-specific event
func (m *StreamMonitor) broadcastMarkerEvent(taskID, eventType string, data map[string]interface{}) {
	if m.wsHub == nil {
		return
	}

	data["task_id"] = taskID
	m.wsHub.Broadcast(broadcast.TaskChannel(taskID), eventType, data)
}

// MonitorBackgroundTask monitors a background task that's already running
// It uses tail -f to stream output in real-time until the exit_code marker is seen
func (m *StreamMonitor) MonitorBackgroundTask(
	ctx context.Context,
	conn *Connection,
	taskID string,
	pid string,
	onUpdate func(output string, status string),
	onComplete func(result *StreamResult),
) error {
	// Check if already streaming this task
	m.mu.Lock()
	if _, exists := m.activeStreams[taskID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("already monitoring task %s", taskID)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	m.activeStreams[taskID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activeStreams, taskID)
		m.mu.Unlock()
	}()

	// Create SSH client for streaming
	sshClient, err := conn.Dial()
	if err != nil {
		m.logger.Error().Err(err).
			Str("task_id", taskID).
			Str("host", conn.Host).
			Msg("MonitorBackgroundTask: failed to connect via SSH")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	m.logger.Info().
		Str("task_id", taskID).
		Str("pid", pid).
		Str("host", conn.Host).
		Bool("has_onUpdate", onUpdate != nil).
		Bool("has_onComplete", onComplete != nil).
		Msg("MonitorBackgroundTask: started real-time streaming")

	taskPaths := paths.GetTaskPaths(conn.GetScriptPath(), taskID)

	// Wait for log file to exist (task may not have started writing yet)
	// Timeout after 30 seconds to avoid hanging forever
	waitCmd := fmt.Sprintf("timeout 30 bash -c 'while [ ! -f %s ]; do sleep 0.1; done'; echo 'ready'", taskPaths.Output)
	m.logger.Debug().
		Str("task_id", taskID).
		Str("output_path", taskPaths.Output).
		Msg("MonitorBackgroundTask: waiting for output file to exist")

	if _, err := sshClient.Run(streamCtx, waitCmd); err != nil {
		if streamCtx.Err() != nil {
			return streamCtx.Err()
		}
		m.logger.Warn().Err(err).Str("task_id", taskID).Msg("Log file wait timed out or failed, continuing anyway")
	}

	// Use tail -f to stream the log file in real-time
	// -n +1 starts from the beginning of the file
	tailCmd := fmt.Sprintf("tail -n +1 -f %s 2>/dev/null", taskPaths.Output)

	m.logger.Info().
		Str("task_id", taskID).
		Str("output_path", taskPaths.Output).
		Str("tail_cmd", tailCmd).
		Msg("MonitorBackgroundTask: starting tail -f streaming")

	var outputBuffer strings.Builder
	var lastBroadcast time.Time
	var exitCode int
	var finalStatus string

	lineCount := 0
	err = sshClient.StreamOutput(streamCtx, tailCmd, func(line string) error {
		lineCount++
		if lineCount <= 5 || lineCount%100 == 0 {
			m.logger.Debug().
				Str("task_id", taskID).
				Int("line_count", lineCount).
				Msg("MonitorBackgroundTask: received line from tail -f")
		}

		// Check for markers - process them via handleMarker (same as StreamTaskOutput)
		if marker := markers.Parse(line); marker != nil {
			m.logger.Info().
				Str("task_id", taskID).
				Str("marker_type", string(marker.Type)).
				Str("marker_value", marker.Value).
				Msg("MonitorBackgroundTask: detected marker")

			// Use handleMarker to process marker and invoke marker handler callbacks
			if err := m.handleMarker(streamCtx, taskID, marker, &exitCode, &finalStatus); err != nil {
				return err
			}

			// For non-exit markers, also call onUpdate with full output
			outputBuffer.WriteString(line + "\n")
			if onUpdate != nil {
				onUpdate(outputBuffer.String(), "running")
			}
			m.broadcastOutput(taskID, outputBuffer.String(), "running")
			lastBroadcast = time.Now()
			return nil
		}

		// Regular output line - add to buffer
		outputBuffer.WriteString(line + "\n")

		// Broadcast accumulated output periodically (every 2 seconds)
		if time.Since(lastBroadcast) >= m.broadcastInterval {
			output := outputBuffer.String()
			if onUpdate != nil {
				onUpdate(output, "running")
			}
			m.broadcastOutput(taskID, output, "running")
			lastBroadcast = time.Now()
		}

		return nil
	})

	// Handle completion
	result := &StreamResult{
		TaskID:     taskID,
		Output:     outputBuffer.String(),
		ExitCode:   exitCode,
		Status:     finalStatus,
		FinishedAt: time.Now(),
	}

	if err != nil && !errors.Is(err, ErrTaskCompleted) {
		result.Error = err
		if finalStatus == "" {
			result.Status = "failed"
		}
	}

	// Final broadcast with complete output
	m.broadcastOutput(taskID, result.Output, result.Status)

	if onComplete != nil {
		onComplete(result)
	}

	m.logger.Info().
		Str("task_id", taskID).
		Str("status", result.Status).
		Int("exit_code", result.ExitCode).
		Msg("Background task monitoring completed")

	return nil
}
