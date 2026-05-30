package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypePollDueBackups is the asynq task type for the every-minute
// scheduler poller. Registered in internal/schedule/kernel.go via
// At("*/1 * * * *", ...).
//
// Why a poller and not per-row schedule registration:
//
// dokploy schedules each backup individually with node-schedule at boot
// (and on every create/update). That works in-process where the
// scheduler lives next to the request handler. Our asynq scheduler
// runs in a separate process and re-reads its task list at startup —
// dynamic per-row registration would require either restarting the
// scheduler on every CRUD or maintaining a side channel.
//
// A single every-minute poller side-steps both. It walks the table,
// computes due-ness against the current minute, and dispatches a
// RunBackupJob per row that's due. At cron's standard 1-minute
// resolution this is functionally identical to per-row scheduling,
// just with one extra DB read per minute.
const TypePollDueBackups = "docker:poll_due_backups"

// PollDueBackupsPayload is empty — the poller walks every enabled
// backup row each tick.
type PollDueBackupsPayload struct{}

// PollDueBackupsJob is the scheduler tick.
type PollDueBackupsJob struct {
	Deps    *JobDeps
	Payload PollDueBackupsPayload
}

// NewPollDueBackupsJob is the asynq constructor (see register.go).
func NewPollDueBackupsJob(p PollDueBackupsPayload) pkgjobs.Handler {
	return &PollDueBackupsJob{Deps: deps, Payload: p}
}

// Handle walks enabled backups and dispatches a RunBackupJob for each
// one due in the current minute. Errors on individual rows are logged
// but don't abort the loop — one broken cron expression shouldn't kill
// every other team's schedule.
func (j *PollDueBackupsJob) Handle(ctx context.Context) error {
	backups, err := j.Deps.Repos.Backup().ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled backups: %w", err)
	}
	if len(backups) == 0 {
		return nil
	}

	// Resolve the minute window once. A backup is "due in this minute"
	// when the next-fire after one minute ago lies within
	// [currentMinute, currentMinute+1m). This means we don't double-fire
	// when ticks are slightly off the minute boundary AND we don't
	// silently drop schedules when the poller runs a few seconds late.
	now := time.Now().UTC()
	startOfMinute := now.Truncate(time.Minute)
	endOfMinute := startOfMinute.Add(time.Minute)

	dispatched := 0
	for i := range backups {
		b := &backups[i]
		if b.CronSchedule == nil || *b.CronSchedule == "" {
			continue
		}
		due, err := cronDueInWindow(*b.CronSchedule, startOfMinute, endOfMinute)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", b.ID).
				Str("cron_schedule", *b.CronSchedule).
				Msg("invalid cron expression on backup; skipping")
			continue
		}
		if !due {
			continue
		}

		// Look up the database to resolve project_id for the run task.
		// Cheap (small table, indexed) and saves us putting project_id
		// on docker_database_backups.
		db, err := j.Deps.Repos.Database().FindByID(ctx, b.DatabaseID)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", b.ID).
				Str("database_id", b.DatabaseID).
				Msg("scheduled backup: database row missing; skipping")
			continue
		}

		task, err := NewRunBackupTask(
			b.ID, b.DatabaseID, db.ProjectID, db.ServerID, b.TeamID, "", "schedule",
		)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", b.ID).
				Msg("scheduled backup: failed to build asynq task")
			continue
		}
		if _, err := j.Deps.Queue.Enqueue(task); err != nil {
			// TaskID conflicts are benign — they mean the same backup
			// was already dispatched this minute (e.g. by a duplicate
			// scheduler tick after a restart). asynq returns a typed
			// error for this; log at debug, not warn.
			if isDuplicateTaskError(err) {
				j.Deps.Logger.Debug().
					Str("backup_id", b.ID).
					Msg("scheduled backup already dispatched this minute")
				continue
			}
			j.Deps.Logger.Warn().Err(err).
				Str("backup_id", b.ID).
				Msg("scheduled backup: failed to enqueue run task")
			continue
		}
		dispatched++
	}

	if dispatched > 0 {
		j.Deps.Logger.Info().
			Int("dispatched", dispatched).
			Int("considered", len(backups)).
			Msg("dispatched scheduled backups")
	}
	return nil
}

// Failed is asynq's framework-level callback. Handle traps row-level
// errors itself so this only fires for genuinely transient failures
// (DB unreachable, etc.).
func (j *PollDueBackupsJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Msg("poll-due-backups job failed at the framework level")
}

// NewPollDueBackupsTask packages the asynq task for the scheduler.
// Signature matches what schedule/kernel.go expects (no-arg constructor
// returning *asynq.Task + error).
func NewPollDueBackupsTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypePollDueBackups, PollDueBackupsPayload{})
}

// cronDueInWindow returns true when the given cron expression fires at
// any moment in [windowStart, windowEnd). The window is closed on the
// left, open on the right — for the typical 1-minute window this gives
// "did this expression fire during this minute?" semantics that mirror
// dokploy's node-schedule check without double-firing on boundary ticks.
//
// Extracted out of Handle so the cron-matching logic can be tested
// without standing up the full job + DB harness.
func cronDueInWindow(expr string, windowStart, windowEnd time.Time) (bool, error) {
	// Standard 5-field cron (minute hour dom month dow) — no descriptors,
	// no seconds, matching the cron syntax shown in the UI's helper text.
	parser := cronParser
	schedule, err := parser.Parse(expr)
	if err != nil {
		return false, err
	}
	// Next() returns the *next* fire strictly after the supplied time,
	// so we feed it (windowStart - 1ns) to ensure a fire exactly at
	// windowStart counts as inside the window.
	next := schedule.Next(windowStart.Add(-time.Nanosecond))
	return !next.Before(windowStart) && next.Before(windowEnd), nil
}

// cronParser is a package-level instance to keep allocations off the
// hot path — the parser itself is stateless and goroutine-safe.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// isDuplicateTaskError checks for asynq's task-already-exists sentinel.
// asynq returns this when TaskID dedup catches a redispatch — for the
// backup poller, that's a perfectly normal occurrence after a restart.
func isDuplicateTaskError(err error) bool {
	return errors.Is(err, asynq.ErrTaskIDConflict)
}
