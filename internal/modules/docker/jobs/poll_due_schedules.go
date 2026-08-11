package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/pkg/cronutil"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// TypePollDueSchedules is the asynq task type for the every-minute
// docker-application-schedules poller. Registered in
// internal/schedule/kernel.go via At("*/1 * * * *", ...).
//
// Why a single poller instead of per-row registration in asynq's
// scheduler: see poll_due_backups.go — same rationale. asynq's
// Scheduler reads its task list at process start, so dynamic per-row
// registration would need a restart on every CRUD. A 1-minute
// poller side-steps that.
//
// Mirrors dokploy's initSchedules() walk (re-reading the DB at boot)
// but spreads it across every minute instead of once-at-boot, so a
// schedule created mid-run picks up on the next tick rather than the
// next worker restart.
const TypePollDueSchedules = "docker:poll_due_schedules"

// PollDueSchedulesPayload is empty — the poller walks every enabled
// schedule each tick.
type PollDueSchedulesPayload struct{}

// PollDueSchedulesJob is the scheduler tick.
type PollDueSchedulesJob struct {
	Deps    *JobDeps
	Payload PollDueSchedulesPayload
}

// NewPollDueSchedulesJob is the asynq constructor (see register.go).
func NewPollDueSchedulesJob(p PollDueSchedulesPayload) pkgjobs.Handler {
	return &PollDueSchedulesJob{Deps: deps, Payload: p}
}

// Handle walks enabled application schedules and dispatches a
// RunApplicationScheduleJob for each row whose cron fires in the
// current minute. Per-row errors are logged but never abort the
// loop — one team's broken cron shouldn't kill the others'.
func (j *PollDueSchedulesJob) Handle(ctx context.Context) error {
	schedules, err := j.Deps.Repos.Schedule().ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil
	}

	now := time.Now().UTC()
	startOfMinute := now.Truncate(time.Minute)
	endOfMinute := startOfMinute.Add(time.Minute)

	dispatched := 0
	for i := range schedules {
		s := &schedules[i]
		if s.Cron == "" {
			continue
		}
		due, err := cronutil.DueInWindow(s.Cron, startOfMinute, endOfMinute)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("schedule_id", s.ID).
				Str("cron", s.Cron).
				Msg("invalid cron expression on application schedule; skipping")
			continue
		}
		if !due {
			continue
		}

		task, err := NewRunApplicationScheduleTask(s.ID)
		if err != nil {
			j.Deps.Logger.Warn().Err(err).
				Str("schedule_id", s.ID).
				Msg("scheduled run: failed to build asynq task")
			continue
		}
		if _, err := j.Deps.Queue.Enqueue(task); err != nil {
			if isDuplicateTaskError(err) {
				// Same row already dispatched this minute — fine. This
				// is the same dedup the backup poller relies on after a
				// scheduler restart.
				j.Deps.Logger.Debug().
					Str("schedule_id", s.ID).
					Msg("scheduled run already dispatched this minute")
				continue
			}
			j.Deps.Logger.Warn().Err(err).
				Str("schedule_id", s.ID).
				Msg("scheduled run: failed to enqueue task")
			continue
		}
		dispatched++
	}

	if dispatched > 0 {
		j.Deps.Logger.Info().
			Int("dispatched", dispatched).
			Int("considered", len(schedules)).
			Msg("dispatched scheduled application runs")
	}
	return nil
}

// Failed is asynq's framework-level callback. Handle traps row-level
// errors itself so this only fires for genuinely transient failures.
func (j *PollDueSchedulesJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Msg("poll-due-schedules job failed at the framework level")
}

// NewPollDueSchedulesTask packages the asynq task for the scheduler.
// Signature matches what schedule/kernel.go expects.
func NewPollDueSchedulesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypePollDueSchedules, PollDueSchedulesPayload{})
}
