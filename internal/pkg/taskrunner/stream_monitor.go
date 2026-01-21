package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Task status markers that scripts should output
const (
	MarkerTaskStarted  = "::LAUNCH_TASK_STARTED::"
	MarkerTaskFinished = "::LAUNCH_TASK_FINISHED::"
	MarkerTaskFailed   = "::LAUNCH_TASK_FAILED::"
	MarkerTaskProgress = "::LAUNCH_TASK_PROGRESS::"
	MarkerTaskStatus   = "::LAUNCH_TASK_STATUS::"
	MarkerExitCode     = "::LAUNCH_EXIT_CODE::"
)

// ErrTaskCompleted signals that the task has completed (not an error)
var ErrTaskCompleted = errors.New("task completed")

// StreamMonitor monitors task output via persistent SSH connection
type StreamMonitor struct {
	logger            *zerolog.Logger
	wsHub             SimpleBroadcaster
	broadcastInterval time.Duration
	activeStreams     map[string]context.CancelFunc
	mu                sync.RWMutex
}

// StreamMonitorConfig holds configuration for the stream monitor
type StreamMonitorConfig struct {
	BroadcastInterval time.Duration // How often to broadcast accumulated output
}

// NewStreamMonitor creates a new stream monitor
func NewStreamMonitor(logger *zerolog.Logger, wsHub SimpleBroadcaster, cfg *StreamMonitorConfig) *StreamMonitor {
	interval := 2 * time.Second
	if cfg != nil && cfg.BroadcastInterval > 0 {
		interval = cfg.BroadcastInterval
	}

	return &StreamMonitor{
		logger:            logger,
		wsHub:             wsHub,
		broadcastInterval: interval,
		activeStreams:     make(map[string]context.CancelFunc),
	}
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
	logFile := fmt.Sprintf("%s/task-%s.log", conn.GetScriptPath(), taskID)

	// Wait for log file to exist (task may not have started writing yet)
	waitCmd := fmt.Sprintf("while [ ! -f %s ]; do sleep 0.5; done; echo 'ready'", logFile)
	if _, err := sshClient.Run(streamCtx, waitCmd); err != nil {
		if streamCtx.Err() != nil {
			return streamCtx.Err()
		}
		m.logger.Warn().Err(err).Str("task_id", taskID).Msg("Log file not found, continuing anyway")
	}

	// Use tail -f to stream the log file
	// -n +1 starts from the beginning of the file
	tailCmd := fmt.Sprintf("tail -n +1 -f %s 2>/dev/null", logFile)

	var outputBuffer strings.Builder
	var lastBroadcast time.Time
	var exitCode int
	var finalStatus string

	// Pattern matchers for markers
	progressPattern := regexp.MustCompile(MarkerTaskProgress + `(\d+)`)
	exitCodePattern := regexp.MustCompile(MarkerExitCode + `(\d+)`)

	err = sshClient.StreamOutput(streamCtx, tailCmd, func(line string) error {
		outputBuffer.WriteString(line + "\n")

		// Check for completion markers
		if strings.Contains(line, MarkerTaskFinished) {
			finalStatus = "finished"
			// Try to extract exit code
			if matches := exitCodePattern.FindStringSubmatch(line); len(matches) > 1 {
				exitCode, _ = strconv.Atoi(matches[1])
			}
			return ErrTaskCompleted
		}

		if strings.Contains(line, MarkerTaskFailed) {
			finalStatus = "failed"
			if matches := exitCodePattern.FindStringSubmatch(line); len(matches) > 1 {
				exitCode, _ = strconv.Atoi(matches[1])
			} else {
				exitCode = 1
			}
			return ErrTaskCompleted
		}

		// Check for progress updates
		if matches := progressPattern.FindStringSubmatch(line); len(matches) > 1 {
			progress, _ := strconv.Atoi(matches[1])
			m.broadcastProgress(taskID, progress)
		}

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

	m.wsHub.Broadcast("task."+taskID, "task.output", map[string]interface{}{
		"task_id": taskID,
		"output":  output,
		"status":  status,
	})
}

func (m *StreamMonitor) broadcastProgress(taskID string, progress int) {
	if m.wsHub == nil {
		return
	}

	m.wsHub.Broadcast("task."+taskID, "task.progress", map[string]interface{}{
		"task_id":  taskID,
		"progress": progress,
	})
}

// MonitorBackgroundTask monitors a background task that's already running
// It polls the process status and streams output until completion
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

	// Create SSH client - this single connection will be used throughout
	sshClient, err := conn.Dial()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	m.logger.Info().
		Str("task_id", taskID).
		Str("pid", pid).
		Str("host", conn.Host).
		Msg("Started monitoring background task")

	logFile := fmt.Sprintf("%s/task-%s.log", conn.GetScriptPath(), taskID)
	exitCodeFile := fmt.Sprintf("%s/task-%s.exit", conn.GetScriptPath(), taskID)

	var lastOutput string
	checkInterval := 2 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-streamCtx.Done():
			return streamCtx.Err()

		case <-ticker.C:
			// Check if process is still running
			psResult, _ := sshClient.Run(streamCtx, fmt.Sprintf("ps -p %s -o pid= 2>/dev/null", pid))
			isRunning := strings.TrimSpace(psResult.Stdout) != ""

			// Get current output
			output, err := sshClient.Download(streamCtx, logFile)
			if err == nil {
				currentOutput := string(output)
				if currentOutput != lastOutput {
					lastOutput = currentOutput
					if onUpdate != nil {
						onUpdate(currentOutput, "running")
					}
					m.broadcastOutput(taskID, currentOutput, "running")
				}
			}

			if !isRunning {
				// Process finished - get exit code
				var exitCode int
				var status string

				// Try to read exit code from file (if script wrote it)
				exitResult, err := sshClient.Run(streamCtx, fmt.Sprintf("cat %s 2>/dev/null", exitCodeFile))
				if err == nil && exitResult.Stdout != "" {
					exitCode, _ = strconv.Atoi(strings.TrimSpace(exitResult.Stdout))
				}

				// Also check for markers in output
				if strings.Contains(lastOutput, MarkerTaskFinished) {
					status = "finished"
				} else if strings.Contains(lastOutput, MarkerTaskFailed) {
					status = "failed"
				} else if exitCode == 0 {
					status = "finished"
				} else if exitCode == 124 {
					status = "timeout"
				} else {
					status = "failed"
				}

				result := &StreamResult{
					TaskID:     taskID,
					Output:     lastOutput,
					ExitCode:   exitCode,
					Status:     status,
					FinishedAt: time.Now(),
				}

				m.broadcastOutput(taskID, lastOutput, status)

				if onComplete != nil {
					onComplete(result)
				}

				m.logger.Info().
					Str("task_id", taskID).
					Str("status", status).
					Int("exit_code", exitCode).
					Msg("Background task monitoring completed")

				return nil
			}
		}
	}
}
