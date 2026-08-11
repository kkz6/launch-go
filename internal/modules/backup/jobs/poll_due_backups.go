package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/cronutil"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypePollDueBackups = "backup:poll_due_backups"

type PollDueBackupsPayload struct{}

type PollDueBackupsJob struct {
	Deps    *JobDeps
	Payload PollDueBackupsPayload
	now     func() time.Time
	enqueue func(*asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

func NewPollDueBackupsJob(payload PollDueBackupsPayload) pkgjobs.Handler {
	return &PollDueBackupsJob{Deps: deps, Payload: payload}
}

func (j *PollDueBackupsJob) Handle(ctx context.Context) error {
	backups, err := j.Deps.Repos.Backup().ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled backups: %w", err)
	}

	now := time.Now().UTC()
	if j.now != nil {
		now = j.now().UTC()
	}
	minute := now.Truncate(time.Minute)

	for i := range backups {
		backup := &backups[i]
		due, parseErr := cronutil.DueInWindow(
			backup.CronExpression,
			minute,
			minute.Add(time.Minute),
		)
		if parseErr != nil {
			j.Deps.Logger.Warn().Err(parseErr).
				Str("backup_id", backup.ID).
				Str("cron_expression", backup.CronExpression).
				Msg("invalid server backup cron expression; skipping")
			continue
		}
		if !due {
			continue
		}

		j.dispatchScheduledBackup(ctx, backup, minute)
	}

	return nil
}

func (j *PollDueBackupsJob) dispatchScheduledBackup(
	ctx context.Context,
	backup *models.Backup,
	minute time.Time,
) {
	run := &models.BackupJob{
		Status:            backuptypes.BackupJobStatusPending,
		BackupID:          backup.ID,
		StorageProviderID: backup.StorageProviderID,
	}
	run.TeamID = backup.TeamID
	if err := j.Deps.Repos.BackupJob().CreateBackupJob(ctx, run); err != nil {
		j.Deps.Logger.Warn().Err(err).Str("backup_id", backup.ID).
			Msg("scheduled backup: failed to create run")
		return
	}

	task, err := NewRunScheduledBackupTask(
		backup.ServerID,
		backup.ID,
		backup.TeamID,
		run.ID,
		minute,
	)
	if err != nil {
		j.failRun(ctx, backup, run, "failed to build scheduled backup task: "+err.Error())
		return
	}

	enqueue := j.enqueue
	if enqueue == nil && j.Deps.Queue != nil {
		enqueue = j.Deps.Queue.Enqueue
	}
	if enqueue == nil {
		j.failRun(ctx, backup, run, "backup queue is not configured")
		return
	}

	if _, err := enqueue(task); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			if deleteErr := j.Deps.Repos.BackupJob().DeleteBackupJob(ctx, run.ID); deleteErr != nil {
				j.failRun(ctx, backup, run, "scheduled backup was already dispatched")
			}
			return
		}
		j.failRun(ctx, backup, run, "failed to enqueue scheduled backup: "+err.Error())
		return
	}

	j.Deps.BroadcastToTeam(backup.TeamID, "backup.run.queued", map[string]any{
		"backup_id": backup.ID,
		"server_id": backup.ServerID,
		"team_id":   backup.TeamID,
		"job_id":    run.ID,
		"status":    string(backuptypes.BackupJobStatusPending),
		"source":    "schedule",
	})
}

func (j *PollDueBackupsJob) failRun(
	ctx context.Context,
	backup *models.Backup,
	run *models.BackupJob,
	message string,
) {
	message = strings.TrimSpace(message)
	updated, err := j.Deps.Repos.BackupJob().MarkBackupJobFailedForRun(
		ctx,
		run.ID,
		backup.ID,
		backup.TeamID,
		message,
		nil,
	)
	if err != nil || !updated {
		j.Deps.Logger.Error().Err(err).Str("backup_id", backup.ID).Str("job_id", run.ID).
			Msg("scheduled backup: failed to persist dispatch failure")
		return
	}
	j.Deps.BroadcastToTeam(backup.TeamID, "backup.run.failed", map[string]any{
		"backup_id": backup.ID,
		"server_id": backup.ServerID,
		"team_id":   backup.TeamID,
		"job_id":    run.ID,
		"error":     message,
		"source":    "schedule",
	})
}

func (j *PollDueBackupsJob) Failed(_ context.Context, err error) {
	j.Deps.Logger.Error().Err(err).Msg("server backup scheduler poll failed")
}

func NewPollDueBackupsTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypePollDueBackups, PollDueBackupsPayload{})
}
