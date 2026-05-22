package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationSchedule runs a command inside the application's container
// on a cron schedule. We schedule via a host-side cron entry that does
// `docker exec <container> <cmd>` — keeps the schedule independent of
// the container's restarts.
//
// (application_id, name) is intentionally NOT enforced unique — users
// may want multiple schedules with the same generic name like "cleanup".
// The id is the stable identifier.
type ApplicationSchedule struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string     `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Cron          string     `gorm:"type:varchar(255);not null" json:"cron"`
	Command       string     `gorm:"type:text;not null" json:"command"`
	LastRunAt     *time.Time `gorm:"column:last_run_at;type:timestamp null" json:"last_run_at,omitempty"`
	LastStatus    *string    `gorm:"column:last_status;type:varchar(32)" json:"last_status,omitempty"`
}

func (ApplicationSchedule) TableName() string { return "docker_application_schedules" }
