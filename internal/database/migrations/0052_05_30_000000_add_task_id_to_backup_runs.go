package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds task_id to docker_database_backup_runs so a manual/scheduled
// backup run links to the server-tasks row the worker creates. The
// frontend uses it (with the /terminal/logs?entity=task websocket) to
// stream the live dump/upload output via ServerLogViewer while the
// backup runs — the same pattern docker deployments use. NULL for
// historical runs (which predate task tracking) and for runs that
// failed before a task was dispatched.
func init() {
	Register(Migration{
		ID:        "0052_05_30_000000_add_task_id_to_backup_runs",
		Name:      "Add task_id to docker_database_backup_runs",
		Timestamp: time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		Up:        addTaskIDToBackupRunsUp,
		Down:      addTaskIDToBackupRunsDown,
	})
}

func addTaskIDToBackupRunsUp(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE docker_database_backup_runs ADD COLUMN IF NOT EXISTS task_id CHAR(26) NULL`).Error
}

func addTaskIDToBackupRunsDown(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE docker_database_backup_runs DROP COLUMN IF EXISTS task_id`).Error
}
