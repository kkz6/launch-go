package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0031_05_23_000006_enrich_application_schedules",
		Name:      "Add enabled / shell_type / last_task_id to docker_application_schedules",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 6, 0, time.UTC),
		Up:        enrichApplicationSchedulesUp,
		Down:      enrichApplicationSchedulesDown,
	})
}

// enrichApplicationSchedulesUp brings the docker schedules row in line
// with dokploy's schedule shape so the worker has everything it needs
// to fire the cron without re-deriving context:
//
//   - enabled       BOOL    — pause without delete (dokploy
//     `schedule.enabled`)
//   - shell_type    VARCHAR — bash vs sh, mirrors the per-container
//     shell picker in the terminal modal
//   - last_task_id  CHAR(26)— ULID of the server-tasks row from the
//     most recent run. The Schedules tab's
//     View Logs button reads its log path
//     via the existing ServerLogViewer
//     entity="task" stream.
func enrichApplicationSchedulesUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_schedules ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_schedules ADD COLUMN shell_type VARCHAR(16) NOT NULL DEFAULT 'sh'",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_schedules ADD COLUMN last_task_id CHAR(26) NULL",
	).Error
}

func enrichApplicationSchedulesDown(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_schedules DROP COLUMN last_task_id",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_schedules DROP COLUMN shell_type",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_schedules DROP COLUMN enabled",
	).Error
}
