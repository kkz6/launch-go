package taskrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// SimpleBroadcaster is a minimal interface for broadcasting.
// This is satisfied by broadcast.Broadcaster, broadcast.NopBroadcaster, and any
// other type that implements the Broadcast method.
// This minimal interface allows the taskrunner package to work independently
// while accepting any broadcaster from the broadcast package.
type SimpleBroadcaster interface {
	Broadcast(channel string, event string, data any)
}

// TaskDispatcher interface defines the contract for task dispatchers.
// Both Dispatcher and FakeDispatcher implement this interface.
type TaskDispatcher interface {
	Run(ctx context.Context, pt *PendingTask) (*TaskResult, error)
}

// Dispatcher handles task execution
type Dispatcher struct {
	logger        *zerolog.Logger
	ws            SimpleBroadcaster
	streamMonitor *StreamMonitor
	localMode     bool
}

// DispatcherConfig holds configuration for the dispatcher
type DispatcherConfig struct {
	LocalMode         bool          // Use SSH streaming instead of HTTP callbacks
	BroadcastInterval time.Duration // How often to broadcast output updates
}

// NewDispatcher creates a new task dispatcher
func NewDispatcher(logger *zerolog.Logger, ws SimpleBroadcaster) *Dispatcher {
	return &Dispatcher{
		logger:    logger,
		ws:        ws,
		localMode: false,
	}
}

// NewDispatcherWithConfig creates a new dispatcher with configuration
func NewDispatcherWithConfig(logger *zerolog.Logger, ws SimpleBroadcaster, cfg *DispatcherConfig) *Dispatcher {
	d := &Dispatcher{
		logger:    logger,
		ws:        ws,
		localMode: cfg != nil && cfg.LocalMode,
	}

	// Initialize stream monitor if in local mode
	if d.localMode {
		d.streamMonitor = NewStreamMonitor(logger, ws, &StreamMonitorConfig{
			BroadcastInterval: cfg.BroadcastInterval,
		})
	}

	return d
}

// IsLocalMode returns whether the dispatcher is in local mode
func (d *Dispatcher) IsLocalMode() bool {
	return d.localMode
}

// GetStreamMonitor returns the stream monitor (may be nil if not in local mode)
func (d *Dispatcher) GetStreamMonitor() *StreamMonitor {
	return d.streamMonitor
}

// Run executes a pending task
func (d *Dispatcher) Run(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	if pt.Connection != nil {
		return d.runRemote(ctx, pt)
	}
	return d.runLocal(ctx, pt)
}

// runLocal executes task on local machine
func (d *Dispatcher) runLocal(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	script := pt.Task.Script()

	// Create temp script file
	tmpFile, err := os.CreateTemp("", "task-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	tmpFile.Close()

	// Create context with timeout
	timeout := pt.Task.Timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()

	cmd := exec.CommandContext(ctx, "bash", tmpFile.Name())
	output, err := cmd.CombinedOutput()

	result := &TaskResult{
		TaskID:     pt.TaskID,
		Output:     string(output),
		Duration:   time.Since(startTime),
		FinishedAt: time.Now(),
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = 124
		pt.Task.OnTimeout(ctx, result)
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err
			result.ExitCode = 1
		}
		pt.Task.OnFailed(ctx, result)
		return result, nil
	}

	pt.Task.OnFinished(ctx, result)
	return result, nil
}

// runRemote executes task on remote server via SSH
func (d *Dispatcher) runRemote(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	script := pt.Task.Script()

	conn := pt.Connection

	// Create SSH client
	sshClient, err := conn.Dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	scriptPath := conn.GetScriptPath()
	taskID := pt.TaskID
	if taskID == "" {
		taskID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	scriptFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.sh", taskID))
	outputFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.log", taskID))

	// Ensure script directory exists
	if _, err := sshClient.Run(ctx, fmt.Sprintf("mkdir -p %s", scriptPath)); err != nil {
		return nil, fmt.Errorf("failed to create script directory: %w", err)
	}

	// Upload script
	if err := sshClient.Upload(ctx, []byte(script), scriptFile, 0755); err != nil {
		return nil, fmt.Errorf("failed to upload script: %w", err)
	}

	startTime := time.Now()

	if pt.Background {
		return d.runRemoteBackground(ctx, sshClient, pt, scriptFile, outputFile, startTime)
	}

	return d.runRemoteForeground(ctx, sshClient, pt, scriptFile, outputFile, startTime)
}

func (d *Dispatcher) runRemoteForeground(
	ctx context.Context,
	client *SSHClient,
	pt *PendingTask,
	scriptFile, outputFile string,
	startTime time.Time,
) (*TaskResult, error) {
	timeout := int(pt.Task.Timeout().Seconds())

	// Run with timeout and tee output
	command := fmt.Sprintf("timeout %ds bash %s 2>&1 | tee %s; exit ${PIPESTATUS[0]}",
		timeout, scriptFile, outputFile)

	cmdResult, err := client.Run(ctx, command)

	result := &TaskResult{
		TaskID:     pt.TaskID,
		Duration:   time.Since(startTime),
		FinishedAt: time.Now(),
	}

	if cmdResult != nil {
		result.Output = cmdResult.Stdout + cmdResult.Stderr
		result.ExitCode = cmdResult.ExitCode
	}

	if err != nil {
		result.Error = err
	}

	// Exit code 124 means timeout
	if result.ExitCode == 124 {
		result.TimedOut = true
		pt.Task.OnTimeout(ctx, result)
		return result, nil
	}

	if result.ExitCode != 0 || result.Error != nil {
		pt.Task.OnFailed(ctx, result)
		return result, nil
	}

	pt.Task.OnFinished(ctx, result)
	return result, nil
}

func (d *Dispatcher) runRemoteBackground(
	ctx context.Context,
	client *SSHClient,
	pt *PendingTask,
	scriptFile, outputFile string,
	startTime time.Time,
) (*TaskResult, error) {
	timeout := int(pt.Task.Timeout().Seconds())

	// Write exit code to file when script completes (for local mode monitoring)
	exitCodeFile := strings.TrimSuffix(outputFile, ".log") + ".exit"

	// Background execution with nohup
	// Also write exit code to a file for SSH polling to detect completion
	command := fmt.Sprintf(
		"nohup bash -c 'timeout %ds bash %s > %s 2>&1; echo $? > %s' & echo $!",
		timeout, scriptFile, outputFile, exitCodeFile,
	)

	cmdResult, err := client.Run(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("failed to start background task: %w", err)
	}

	pid := strings.TrimSpace(cmdResult.Stdout)

	d.logger.Info().
		Str("task_id", pt.TaskID).
		Str("pid", pid).
		Str("output_file", outputFile).
		Bool("local_mode", d.localMode).
		Msg("Background task started")

	// In local mode, start SSH streaming to monitor the task
	if d.localMode && d.streamMonitor != nil {
		go func() {
			// Create a new context for streaming (don't use the original which may be cancelled)
			streamCtx, cancel := context.WithTimeout(context.Background(), pt.Task.Timeout()+time.Minute)
			defer cancel()

			err := d.streamMonitor.MonitorBackgroundTask(
				streamCtx,
				pt.Connection,
				pt.TaskID,
				pid,
				func(output string, status string) {
					// Output update callback
					pt.Task.OnOutput(output)
				},
				func(result *StreamResult) {
					// Completion callback
					taskResult := &TaskResult{
						TaskID:     result.TaskID,
						Output:     result.Output,
						ExitCode:   result.ExitCode,
						Duration:   time.Since(startTime),
						FinishedAt: result.FinishedAt,
					}

					switch result.Status {
					case "finished":
						pt.Task.OnFinished(streamCtx, taskResult)
					case "timeout":
						taskResult.TimedOut = true
						pt.Task.OnTimeout(streamCtx, taskResult)
					default:
						pt.Task.OnFailed(streamCtx, taskResult)
					}
				},
			)

			if err != nil {
				d.logger.Error().Err(err).Str("task_id", pt.TaskID).Msg("SSH streaming error")
			}
		}()
	}

	// Return immediately - task will complete asynchronously
	return &TaskResult{
		TaskID:     pt.TaskID,
		Output:     fmt.Sprintf("Background task started with PID: %s", pid),
		Duration:   time.Since(startTime),
		FinishedAt: time.Now(),
	}, nil
}

// RunWithStreaming runs a task on a remote server with live SSH output streaming
// This is useful for local development where HTTP callbacks are not available
func (d *Dispatcher) RunWithStreaming(ctx context.Context, pt *PendingTask) (*TaskResult, error) {
	if pt.Connection == nil {
		return nil, fmt.Errorf("streaming requires a remote connection")
	}

	script := pt.Task.Script()

	conn := pt.Connection

	// Create SSH client
	sshClient, err := conn.Dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()

	scriptPath := conn.GetScriptPath()
	taskID := pt.TaskID
	if taskID == "" {
		taskID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	scriptFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.sh", taskID))
	outputFile := filepath.Join(scriptPath, fmt.Sprintf("task-%s.log", taskID))

	// Ensure script directory exists
	if _, err := sshClient.Run(ctx, fmt.Sprintf("mkdir -p %s", scriptPath)); err != nil {
		return nil, fmt.Errorf("failed to create script directory: %w", err)
	}

	// Upload script
	if err := sshClient.Upload(ctx, []byte(script), scriptFile, 0755); err != nil {
		return nil, fmt.Errorf("failed to upload script: %w", err)
	}

	startTime := time.Now()
	timeout := int(pt.Task.Timeout().Seconds())

	// Run with output streaming
	var outputBuffer strings.Builder
	command := fmt.Sprintf("timeout %ds bash %s 2>&1 | tee %s", timeout, scriptFile, outputFile)

	var exitCode int
	err = sshClient.StreamOutput(ctx, command, func(line string) error {
		outputBuffer.WriteString(line + "\n")

		// Call OnOutput callback for live updates
		pt.Task.OnOutput(outputBuffer.String())

		// Broadcast to WebSocket
		if d.ws != nil {
			d.ws.Broadcast("task."+taskID, "task.output", map[string]interface{}{
				"task_id": taskID,
				"output":  outputBuffer.String(),
				"status":  "running",
			})
		}

		return nil
	})

	result := &TaskResult{
		TaskID:     pt.TaskID,
		Output:     outputBuffer.String(),
		Duration:   time.Since(startTime),
		FinishedAt: time.Now(),
		ExitCode:   exitCode,
	}

	if err != nil {
		// Check if it was a context cancellation
		if ctx.Err() != nil {
			result.Error = ctx.Err()
		} else {
			result.Error = err
		}
	}

	// Determine final status
	if result.ExitCode == 124 {
		result.TimedOut = true
		pt.Task.OnTimeout(ctx, result)
	} else if result.ExitCode != 0 || result.Error != nil {
		pt.Task.OnFailed(ctx, result)
	} else {
		pt.Task.OnFinished(ctx, result)
	}

	return result, nil
}

// GetTaskOutput fetches output from a remote task
func (d *Dispatcher) GetTaskOutput(ctx context.Context, conn *Connection, taskID string) (string, error) {
	sshClient, err := conn.Dial()
	if err != nil {
		return "", err
	}
	defer sshClient.Close()

	outputFile := filepath.Join(conn.GetScriptPath(), fmt.Sprintf("task-%s.log", taskID))
	output, err := sshClient.Download(ctx, outputFile)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// CheckTaskStatus checks if a background task is still running
func (d *Dispatcher) CheckTaskStatus(ctx context.Context, conn *Connection, pid string) (bool, int, error) {
	sshClient, err := conn.Dial()
	if err != nil {
		return false, 0, err
	}
	defer sshClient.Close()

	// Check if process is running
	result, _ := sshClient.Run(ctx, fmt.Sprintf("ps -p %s -o pid= 2>/dev/null", pid))
	isRunning := strings.TrimSpace(result.Stdout) != ""

	if isRunning {
		return true, 0, nil
	}

	// Process finished - try to get exit code from wait
	// This is tricky for background processes, typically need to store in file
	return false, 0, nil
}
