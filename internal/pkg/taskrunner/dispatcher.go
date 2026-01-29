package taskrunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/pkg/launch/paths"

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
}

// DispatcherConfig holds configuration for the dispatcher
type DispatcherConfig struct {
	BroadcastInterval time.Duration // How often to broadcast output updates
}

// NewDispatcher creates a new task dispatcher
func NewDispatcher(logger *zerolog.Logger, ws SimpleBroadcaster) *Dispatcher {
	return &Dispatcher{
		logger:        logger,
		ws:            ws,
		streamMonitor: NewStreamMonitor(logger, ws, nil),
	}
}

// NewDispatcherWithConfig creates a new dispatcher with configuration
func NewDispatcherWithConfig(logger *zerolog.Logger, ws SimpleBroadcaster, cfg *DispatcherConfig) *Dispatcher {
	var broadcastInterval time.Duration
	if cfg != nil {
		broadcastInterval = cfg.BroadcastInterval
	}

	return &Dispatcher{
		logger: logger,
		ws:     ws,
		streamMonitor: NewStreamMonitor(logger, ws, &StreamMonitorConfig{
			BroadcastInterval: broadcastInterval,
		}),
	}
}

// GetStreamMonitor returns the stream monitor
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
	_ = tmpFile.Close()

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

	d.logger.Debug().
		Str("task_id", pt.TaskID).
		Str("host", conn.Host).
		Msg("runRemote: connecting via SSH")

	// Create SSH client
	sshClient, err := conn.Dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	d.logger.Debug().
		Str("task_id", pt.TaskID).
		Msg("runRemote: SSH connected, creating script directory")

	taskDir := conn.GetScriptPath()
	taskID := pt.TaskID
	if taskID == "" {
		taskID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	taskPaths := paths.GetTaskPaths(taskDir, taskID)

	// Ensure script directory exists
	if _, err := sshClient.Run(ctx, fmt.Sprintf("mkdir -p %s", taskDir)); err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("failed to create script directory: %w", err)
	}

	d.logger.Debug().
		Str("task_id", pt.TaskID).
		Str("script_path", taskPaths.Script).
		Msg("runRemote: uploading script")

	// Upload script
	if err := sshClient.Upload(ctx, []byte(script), taskPaths.Script, 0755); err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("failed to upload script: %w", err)
	}

	d.logger.Debug().
		Str("task_id", pt.TaskID).
		Bool("background", pt.Background).
		Msg("runRemote: script uploaded, starting execution")

	startTime := time.Now()

	if pt.Background {
		// For background tasks, the SSH client will be closed by runRemoteBackground
		// after starting the background process. A NEW connection will be created
		// by MonitorBackgroundTask for monitoring.
		return d.runRemoteBackground(ctx, sshClient, pt, taskPaths, startTime)
	}

	// For foreground tasks, close SSH client when done
	defer sshClient.Close()
	return d.runRemoteForeground(ctx, sshClient, pt, taskPaths, startTime)
}

func (d *Dispatcher) runRemoteForeground(
	ctx context.Context,
	client *SSHClient,
	pt *PendingTask,
	taskPaths paths.TaskPaths,
	startTime time.Time,
) (*TaskResult, error) {
	timeout := int(pt.Task.Timeout().Seconds())

	// Run with timeout and tee output
	command := fmt.Sprintf("timeout %ds bash %s 2>&1 | tee %s; exit ${PIPESTATUS[0]}",
		timeout, taskPaths.Script, taskPaths.Output)

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
	taskPaths paths.TaskPaths,
	startTime time.Time,
) (*TaskResult, error) {
	timeout := int(pt.Task.Timeout().Seconds())

	// Background execution with nohup
	// Write exit code to both a file AND append as marker to output for real-time streaming detection
	// Use stdbuf -oL for line buffering so markers are immediately visible to tail -f
	// Without this, stdout is fully buffered when redirected to a file, causing markers
	// to only appear when the buffer is full or the script completes.
	//
	// IMPORTANT: The outer "> /dev/null 2>&1" redirects nohup's own stdout/stderr to /dev/null.
	// Without this, the SSH session holds open waiting for the nohup child's inherited file
	// descriptors to close — which only happens when the script finishes. This causes
	// client.Run() to block for the entire script duration, delaying the monitor goroutine
	// and preventing real-time marker processing.
	// The script's actual output is already captured inside the bash -c via "> taskPaths.Output 2>&1".
	command := fmt.Sprintf(
		"nohup bash -c 'stdbuf -oL timeout %ds bash %s > %s 2>&1; EXIT_CODE=$?; echo $EXIT_CODE > %s; echo \"::LAUNCH::exit_code::$EXIT_CODE\" >> %s' > /dev/null 2>&1 & echo $!",
		timeout, taskPaths.Script, taskPaths.Output, taskPaths.ExitCode, taskPaths.Output,
	)

	d.logger.Debug().
		Str("task_id", pt.TaskID).
		Str("command", command).
		Msg("runRemoteBackground: executing nohup command")

	cmdResult, err := client.Run(ctx, command)

	// Close the SSH client - we're done with it after starting the background process.
	// MonitorBackgroundTask will create its own NEW connection for monitoring.
	_ = client.Close()

	if err != nil {
		return nil, fmt.Errorf("failed to start background task: %w", err)
	}

	pid := strings.TrimSpace(cmdResult.Stdout)

	d.logger.Info().
		Str("task_id", pt.TaskID).
		Str("pid", pid).
		Str("output_file", taskPaths.Output).
		Msg("Background task started")

	// Start SSH streaming to monitor the task
	if d.streamMonitor == nil {
		// If no stream monitor, send immediate result to completion channel
		// to prevent the caller from blocking forever
		if ch := pt.GetCompletionChannel(); ch != nil {
			ch <- &TaskResult{
				TaskID:     pt.TaskID,
				Output:     fmt.Sprintf("Background task started with PID: %s (no monitoring)", pid),
				Duration:   time.Since(startTime),
				FinishedAt: time.Now(),
			}
		}
		return nil, nil
	}

	go func() {
		// Create a new context for streaming (don't use the original which may be cancelled)
		streamCtx, cancel := context.WithTimeout(context.Background(), pt.Task.Timeout()+time.Minute)
		defer cancel()

		d.logger.Info().
			Str("task_id", pt.TaskID).
			Str("pid", pid).
			Bool("has_OnOutput", pt.GetOnOutput() != nil).
			Msg("runRemoteBackground: starting MonitorBackgroundTask goroutine")

		err := d.streamMonitor.MonitorBackgroundTask(
			streamCtx,
			pt.Connection,
			pt.TaskID,
			pid,
			pt.GetMarkerHandler(),
			func(output string, status string) {
				if callback := pt.GetOnOutput(); callback != nil {
					callback(output)
				}
				pt.Task.OnOutput(output)
			},
			func(result *StreamResult) {
				taskResult := result.ToTaskResult()
				taskResult.Duration = time.Since(startTime)

				if ch := pt.GetCompletionChannel(); ch != nil {
					ch <- taskResult
				}

				switch result.Status {
				case "finished":
					pt.Task.OnFinished(streamCtx, taskResult)
				case "timeout":
					pt.Task.OnTimeout(streamCtx, taskResult)
				default:
					pt.Task.OnFailed(streamCtx, taskResult)
				}
			},
		)

		if err != nil {
			d.logger.Error().Err(err).Str("task_id", pt.TaskID).Msg("SSH streaming error")

			// If MonitorBackgroundTask failed before calling onComplete (e.g., SSH connection failure),
			// send a failure result to the completion channel to prevent the caller from blocking forever.
			if ch := pt.GetCompletionChannel(); ch != nil {
				select {
				case ch <- &TaskResult{
					TaskID:     pt.TaskID,
					ExitCode:   1,
					Output:     fmt.Sprintf("SSH monitoring failed: %s", err),
					Duration:   time.Since(startTime),
					FinishedAt: time.Now(),
					Error:      err,
				}:
				default:
					// onComplete already sent a result — no action needed
				}
			}
		}
	}()

	// Return immediately - task will complete asynchronously
	// Return nil result to indicate the task is still running.
	// The actual result will be provided by MonitorBackgroundTask when the task completes.
	return nil, nil
}

// GetTaskOutput fetches output from a remote task
func (d *Dispatcher) GetTaskOutput(ctx context.Context, conn *Connection, taskID string) (string, error) {
	sshClient, err := conn.Dial()
	if err != nil {
		return "", err
	}
	defer sshClient.Close()

	taskPaths := paths.GetTaskPaths(conn.GetScriptPath(), taskID)
	output, err := sshClient.Download(ctx, taskPaths.Output)
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
	result, err := sshClient.Run(ctx, fmt.Sprintf("ps -p %s -o pid= 2>/dev/null", pid))
	if err != nil || result == nil {
		return false, 0, err
	}
	isRunning := strings.TrimSpace(result.Stdout) != ""

	if isRunning {
		return true, 0, nil
	}

	// Process finished - try to get exit code from wait
	// This is tricky for background processes, typically need to store in file
	return false, 0, nil
}
