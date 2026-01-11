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

	"github.com/kkz6/launch-go/internal/ssh"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Dispatcher handles task execution
type Dispatcher struct {
	logger *zerolog.Logger
	ws     *websocket.Hub
}

func NewDispatcher(logger *zerolog.Logger, ws *websocket.Hub) *Dispatcher {
	return &Dispatcher{
		logger: logger,
		ws:     ws,
	}
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
	script, err := pt.Task.Script()
	if err != nil {
		return nil, fmt.Errorf("failed to generate script: %w", err)
	}

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
	script, err := pt.Task.Script()
	if err != nil {
		return nil, fmt.Errorf("failed to generate script: %w", err)
	}

	conn := pt.Connection

	// Create SSH client
	sshClient, err := ssh.NewClient(ssh.Config{
		Host:       conn.Host,
		Port:       conn.Port,
		User:       conn.User,
		PrivateKey: conn.PrivateKey,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH client: %w", err)
	}
	defer sshClient.Close()

	if err := sshClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

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
	client *ssh.Client,
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
	client *ssh.Client,
	pt *PendingTask,
	scriptFile, outputFile string,
	startTime time.Time,
) (*TaskResult, error) {
	timeout := int(pt.Task.Timeout().Seconds())

	// Background execution with nohup
	// The script will call back to our webhook when done
	command := fmt.Sprintf(
		"nohup timeout %ds bash %s > %s 2>&1 & echo $!",
		timeout, scriptFile, outputFile,
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
		Msg("Background task started")

	// Return immediately - task will complete asynchronously
	return &TaskResult{
		TaskID:     pt.TaskID,
		Output:     fmt.Sprintf("Background task started with PID: %s", pid),
		Duration:   time.Since(startTime),
		FinishedAt: time.Now(),
	}, nil
}

// GetTaskOutput fetches output from a remote task
func (d *Dispatcher) GetTaskOutput(ctx context.Context, conn *Connection, taskID string) (string, error) {
	sshClient, err := ssh.NewClient(ssh.Config{
		Host:       conn.Host,
		Port:       conn.Port,
		User:       conn.User,
		PrivateKey: conn.PrivateKey,
	})
	if err != nil {
		return "", err
	}
	defer sshClient.Close()

	if err := sshClient.Connect(); err != nil {
		return "", err
	}

	outputFile := filepath.Join(conn.GetScriptPath(), fmt.Sprintf("task-%s.log", taskID))
	output, err := sshClient.Download(ctx, outputFile)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// CheckTaskStatus checks if a background task is still running
func (d *Dispatcher) CheckTaskStatus(ctx context.Context, conn *Connection, pid string) (bool, int, error) {
	sshClient, err := ssh.NewClient(ssh.Config{
		Host:       conn.Host,
		Port:       conn.Port,
		User:       conn.User,
		PrivateKey: conn.PrivateKey,
	})
	if err != nil {
		return false, 0, err
	}
	defer sshClient.Close()

	if err := sshClient.Connect(); err != nil {
		return false, 0, err
	}

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
