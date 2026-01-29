package tasks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/paths"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// TaskRecoverer recovers orphaned tasks that were running when the worker restarted.
// On startup, it queries for tasks with status "running", SSHes to each server to
// check if the script finished (exit code file exists) or is still running, and
// either finalizes the completed task or re-monitors the still-running one.
type TaskRecoverer struct {
	db          *gorm.DB
	logger      *zerolog.Logger
	dispatcher  *taskrunner.Dispatcher
	queueClient *queue.Client
	broadcaster broadcast.TeamBroadcaster
	notifier    taskrunner.NotifierService
}

// NewTaskRecoverer creates a new TaskRecoverer with the required dependencies.
func NewTaskRecoverer(
	db *gorm.DB,
	logger *zerolog.Logger,
	dispatcher *taskrunner.Dispatcher,
	queueClient *queue.Client,
	broadcaster broadcast.TeamBroadcaster,
	notifier taskrunner.NotifierService,
) *TaskRecoverer {
	return &TaskRecoverer{
		db:          db,
		logger:      logger,
		dispatcher:  dispatcher,
		queueClient: queueClient,
		broadcaster: broadcaster,
		notifier:    notifier,
	}
}

// RecoverOrphanedTasks finds all tasks with status "running" and attempts to recover them.
// It returns nil even if individual task recoveries fail — errors are logged and recovery
// continues for remaining tasks.
func (r *TaskRecoverer) RecoverOrphanedTasks(ctx context.Context) error {
	var tasks []models.Task
	if err := r.db.
		Where("status = ?", string(servertypes.TaskStatusRunning)).
		Preload("Server").
		Find(&tasks).Error; err != nil {
		return fmt.Errorf("failed to query orphaned tasks: %w", err)
	}

	if len(tasks) == 0 {
		r.logger.Info().Msg("Task recovery: no orphaned tasks found")
		return nil
	}

	r.logger.Info().Int("count", len(tasks)).Msg("Task recovery: found orphaned tasks")

	var recovered, failed int
	for i := range tasks {
		task := &tasks[i]

		if task.Server == nil {
			r.logger.Warn().
				Str("task_id", task.ID).
				Str("server_id", task.ServerID).
				Msg("Task recovery: server not found, marking task as failed")
			_ = r.markTaskFailed(task, "server not found during recovery")
			failed++
			continue
		}

		if err := r.recoverTask(ctx, task); err != nil {
			r.logger.Error().Err(err).
				Str("task_id", task.ID).
				Str("server_id", task.ServerID).
				Msg("Task recovery: failed to recover task")
			_ = r.markTaskFailed(task, fmt.Sprintf("recovery failed: %s", err))
			failed++
		} else {
			recovered++
		}
	}

	r.logger.Info().
		Int("total", len(tasks)).
		Int("recovered", recovered).
		Int("failed", failed).
		Msg("Task recovery complete")

	return nil
}

// recoverTask attempts to recover a single orphaned task by SSHing to the server
// and checking if the script has already finished or is still running.
func (r *TaskRecoverer) recoverTask(ctx context.Context, task *models.Task) error {
	conn, err := r.getConnection(task)
	if err != nil {
		return err
	}

	sshClient, err := conn.Dial()
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	taskPaths := paths.GetTaskPaths(conn.GetScriptPath(), task.ID)

	// Check if the exit code file exists (script already finished)
	result, err := sshClient.Run(ctx, fmt.Sprintf("cat %s 2>/dev/null", taskPaths.ExitCode))
	if err == nil && result != nil && strings.TrimSpace(result.Stdout) != "" {
		// Script already finished — read exit code and output, then finalize
		exitCode, parseErr := strconv.Atoi(strings.TrimSpace(result.Stdout))
		if parseErr != nil {
			return fmt.Errorf("failed to parse exit code: %w", parseErr)
		}

		// Read the output log
		output := ""
		outputResult, err := sshClient.Run(ctx, fmt.Sprintf("cat %s 2>/dev/null", taskPaths.Output))
		if err == nil && outputResult != nil {
			output = outputResult.Stdout
		}

		r.logger.Info().
			Str("task_id", task.ID).
			Int("exit_code", exitCode).
			Msg("Task recovery: task already completed, finalizing")

		return r.finalizeCompletedTask(ctx, task, exitCode, output)
	}

	// No exit code file — check if the output log is still being written to
	// (indicates the process is still running)
	stillRunning := false
	checkResult, err := sshClient.Run(ctx, fmt.Sprintf(
		"test -f %s && find %s -mmin -1 -print 2>/dev/null | head -1",
		taskPaths.Output, taskPaths.Output,
	))
	if err == nil && checkResult != nil && strings.TrimSpace(checkResult.Stdout) != "" {
		stillRunning = true
	}

	if stillRunning {
		r.logger.Info().
			Str("task_id", task.ID).
			Msg("Task recovery: task still running, re-monitoring")

		// Close the SSH client before re-monitoring — MonitorBackgroundTask creates its own connection
		sshClient.Close()
		return r.remonitorRunningTask(ctx, task, conn)
	}

	// Output file doesn't exist or hasn't been written to recently — task state is unknown
	r.logger.Warn().
		Str("task_id", task.ID).
		Msg("Task recovery: task state unknown, marking as failed")

	return r.markTaskFailed(task, "task lost during worker restart")
}

// finalizeCompletedTask updates a task that has already finished on the server
// and invokes the appropriate callbacks.
func (r *TaskRecoverer) finalizeCompletedTask(ctx context.Context, task *models.Task, exitCode int, output string) error {
	task.ExitCode = &exitCode
	task.Output = dbtype.EncryptedString(output)

	if exitCode == 0 {
		task.Status = string(servertypes.TaskStatusFinished)
	} else if exitCode == 124 {
		task.Status = string(servertypes.TaskStatusTimeout)
	} else {
		task.Status = string(servertypes.TaskStatusFailed)
	}

	if err := r.db.Save(task).Error; err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Broadcast task completion
	r.broadcastTaskUpdate(task, output)

	// Invoke callbacks
	return r.invokeCallbacks(ctx, task, exitCode)
}

// remonitorRunningTask re-attaches a monitor to a task that is still running on the server.
// It spawns a goroutine that monitors via tail -f and finalizes the task when complete.
func (r *TaskRecoverer) remonitorRunningTask(ctx context.Context, task *models.Task, conn *taskrunner.Connection) error {
	monitor := r.dispatcher.GetStreamMonitor()
	if monitor == nil {
		return fmt.Errorf("stream monitor not available")
	}

	// Capture task data needed by the goroutine
	taskID := task.ID
	taskCopy := *task

	go func() {
		// Use a generous timeout for the re-monitor — use the task's original timeout
		// plus some buffer for the time already elapsed
		timeout := time.Duration(taskCopy.Timeout) * time.Second
		if timeout == 0 {
			timeout = 10 * time.Minute
		}
		// Add extra buffer since some time has already elapsed
		monitorCtx, cancel := context.WithTimeout(context.Background(), timeout+5*time.Minute)
		defer cancel()

		completionChan := make(chan *taskrunner.TaskResult, 1)

		err := monitor.MonitorBackgroundTask(
			monitorCtx,
			conn,
			taskID,
			"",  // PID unknown during recovery — not needed for tail -f monitoring
			nil, // No per-task marker handler during recovery
			nil, // No onUpdate callback during recovery
			func(result *taskrunner.StreamResult) {
				completionChan <- result.ToTaskResult()
			},
		)

		if err != nil {
			r.logger.Error().Err(err).
				Str("task_id", taskID).
				Msg("Task recovery: re-monitor failed")

			// Try to check if the task completed while we were trying to monitor
			recheckCtx, recheckCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer recheckCancel()

			sshClient, sshErr := conn.Dial()
			if sshErr == nil {
				defer sshClient.Close()
				taskPaths := paths.GetTaskPaths(conn.GetScriptPath(), taskID)
				exitResult, exitErr := sshClient.Run(recheckCtx, fmt.Sprintf("cat %s 2>/dev/null", taskPaths.ExitCode))
				if exitErr == nil && exitResult != nil && strings.TrimSpace(exitResult.Stdout) != "" {
					exitCode, parseErr := strconv.Atoi(strings.TrimSpace(exitResult.Stdout))
					if parseErr == nil {
						output := ""
						outputResult, outErr := sshClient.Run(recheckCtx, fmt.Sprintf("cat %s 2>/dev/null", taskPaths.Output))
						if outErr == nil && outputResult != nil {
							output = outputResult.Stdout
						}
						if fErr := r.finalizeCompletedTask(recheckCtx, &taskCopy, exitCode, output); fErr != nil {
							r.logger.Error().Err(fErr).Str("task_id", taskID).Msg("Task recovery: failed to finalize after recheck")
						}
						return
					}
				}
			}

			// Could not recover — mark as failed
			_ = r.markTaskFailed(&taskCopy, fmt.Sprintf("re-monitor failed: %s", err))
			return
		}

		// Wait for completion
		taskResult := <-completionChan

		// Finalize the task
		exitCode := 0
		if taskResult != nil {
			exitCode = taskResult.ExitCode
		}
		output := ""
		if taskResult != nil {
			output = taskResult.Output
		}

		if fErr := r.finalizeCompletedTask(context.Background(), &taskCopy, exitCode, output); fErr != nil {
			r.logger.Error().Err(fErr).
				Str("task_id", taskID).
				Msg("Task recovery: failed to finalize re-monitored task")
		}
	}()

	return nil
}

// invokeCallbacks reconstructs the callback handler from the task's Instance field
// and invokes the appropriate callback (OnSuccess, OnFailure, or OnExpired).
func (r *TaskRecoverer) invokeCallbacks(ctx context.Context, task *models.Task, exitCode int) error {
	instanceData := task.Instance.String()
	if instanceData == "" {
		return nil
	}

	// Try to reconstruct as a CallbackHandler (the Instance field may contain
	// either a TaskInstance with callback data or a CompletionConfig)
	handler, err := taskrunner.ReconstructFromInstance(instanceData)
	if err != nil {
		// May be a CompletionConfig instead — try dispatching completion jobs
		r.dispatchCompletionJobs(task, exitCode, instanceData)
		return nil
	}

	if handler == nil {
		return nil
	}

	cbCtx := &taskrunner.CallbackContext{
		DB:       r.db,
		Queue:    r.queueClient,
		Logger:   r.logger,
		Notifier: r.notifier,
	}
	cbCtx.SetBroadcaster(r.broadcaster)

	timedOut := exitCode == 124 || task.Status == string(servertypes.TaskStatusTimeout)

	if exitCode == 0 {
		if err := handler.OnSuccess(ctx, cbCtx, task.ID); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnSuccess callback failed")
			return err
		}
	} else if timedOut {
		if err := handler.OnExpired(ctx, cbCtx, task.ID); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnExpired callback failed")
			return err
		}
	} else {
		if err := handler.OnFailure(ctx, cbCtx, task.ID, exitCode); err != nil {
			r.logger.Error().Err(err).Str("task_id", task.ID).Msg("Task recovery: OnFailure callback failed")
			return err
		}
	}

	return nil
}

// dispatchCompletionJobs attempts to dispatch completion jobs from a CompletionConfig
// stored in the task's Instance field.
func (r *TaskRecoverer) dispatchCompletionJobs(task *models.Task, exitCode int, instanceData string) {
	config, err := taskrunner.UnmarshalCompletionConfig(instanceData)
	if err != nil || config == nil {
		return
	}

	if r.queueClient == nil {
		return
	}

	var jobRef *taskrunner.JobRef
	if exitCode == 0 {
		jobRef = config.OnFinished
	} else if exitCode == 124 {
		jobRef = config.OnTimeout
	} else {
		jobRef = config.OnFailed
	}

	if jobRef == nil || jobRef.Type == "" {
		return
	}

	r.logger.Info().
		Str("task_id", task.ID).
		Str("job_type", jobRef.Type).
		Msg("Task recovery: dispatching completion job")

	asynqTask := asynq.NewTask(jobRef.Type, jobRef.Payload)
	if _, err := r.queueClient.Enqueue(asynqTask); err != nil {
		r.logger.Error().Err(err).
			Str("task_id", task.ID).
			Str("job_type", jobRef.Type).
			Msg("Task recovery: failed to dispatch completion job")
	}
}

// markTaskFailed updates a task's status to failed with a reason in the output.
func (r *TaskRecoverer) markTaskFailed(task *models.Task, reason string) error {
	exitCode := 1
	task.Status = string(servertypes.TaskStatusFailed)
	task.ExitCode = &exitCode

	// Append recovery failure reason to existing output
	existingOutput := task.Output.String()
	if existingOutput != "" {
		task.Output = dbtype.EncryptedString(existingOutput + "\n[Recovery] " + reason)
	} else {
		task.Output = dbtype.EncryptedString("[Recovery] " + reason)
	}

	if err := r.db.Save(task).Error; err != nil {
		r.logger.Error().Err(err).
			Str("task_id", task.ID).
			Msg("Task recovery: failed to mark task as failed")
		return err
	}

	r.broadcastTaskUpdate(task, reason)

	// Try to invoke failure callbacks even when marking as failed
	_ = r.invokeCallbacks(context.Background(), task, exitCode)

	return nil
}

// broadcastTaskUpdate sends a task update event via WebSocket.
func (r *TaskRecoverer) broadcastTaskUpdate(task *models.Task, output string) {
	if r.broadcaster == nil || task.Server == nil {
		return
	}
	r.broadcaster.BroadcastToTeam(task.Server.TeamID, "task.updated", task.BroadcastData(output))
}

// getConnection returns the appropriate SSH connection for a task.
func (r *TaskRecoverer) getConnection(task *models.Task) (*taskrunner.Connection, error) {
	if task.Server.PublicIPv4 == nil || *task.Server.PublicIPv4 == "" {
		return nil, fmt.Errorf("server %s has no public IP address", task.ServerID)
	}
	if task.Server.PrivateKey.IsEmpty() {
		return nil, fmt.Errorf("server %s has no private key", task.ServerID)
	}

	if task.User == task.Server.RootUsername() {
		return task.Server.ConnectionAsRoot(), nil
	}
	return task.Server.ConnectionAsUser(task.User), nil
}
