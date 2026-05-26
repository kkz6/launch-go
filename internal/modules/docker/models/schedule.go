package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationSchedule runs a command inside the application's container
// on a cron schedule. The schedule itself lives in the worker process
// (registered with asynq.Scheduler at boot) — NOT inside the container
// — so container restart / rebuild is transparent to it. At each tick:
//
//   1. The runner resolves the CURRENT container name from the project
//      + application slugs (so a rebuild that produces a new container
//      ID is picked up without re-arming the schedule).
//   2. SSH's to the host and runs `docker exec <container> <shell> -c
//      '<command>'` via taskrunner.
//   3. Captures stdout+stderr into the taskrunner log file, and stamps
//      the resulting task ID onto LastTaskID for the View Logs UI.
//
// Mirrors dokploy's `runCommand` flow (packages/server/src/utils/
// schedules/utils.ts).
//
// (application_id, name) is intentionally NOT enforced unique — users
// may want multiple schedules with the same generic name like
// "cleanup". The id is the stable identifier.
type ApplicationSchedule struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string     `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Cron          string     `gorm:"type:varchar(255);not null" json:"cron"`
	Command       string     `gorm:"type:text;not null" json:"command"`
	// Enabled lets the user pause a schedule without deleting it.
	// Disabled rows are skipped by the scheduler's boot-time arming
	// pass; mutating this field re-arms the row immediately via the
	// service's scheduler-sync hook.
	Enabled bool `gorm:"column:enabled;not null;default:true" json:"enabled"`
	// ShellType picks the in-container shell to exec into. Same set
	// as the terminal modal: "bash" or "sh". Defaults to "sh" because
	// it's always present (bash isn't in alpine / distroless images).
	ShellType string `gorm:"column:shell_type;type:varchar(16);not null;default:sh" json:"shell_type"`
	// LastTaskID is the server-tasks ULID of the most recent run.
	// View Logs streams from the taskrunner log file at that ID.
	LastTaskID *string    `gorm:"column:last_task_id;type:char(26)" json:"last_task_id,omitempty"`
	LastRunAt  *time.Time `gorm:"column:last_run_at;type:timestamp null" json:"last_run_at,omitempty"`
	LastStatus *string    `gorm:"column:last_status;type:varchar(32)" json:"last_status,omitempty"`
}

func (ApplicationSchedule) TableName() string { return "docker_application_schedules" }
