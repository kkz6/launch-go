package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds task_id to backup_jobs so a server backup run links to the
// server-tasks row the worker creates. The frontend uses it (via the
// /terminal/logs?entity=task websocket) to stream the live tar/upload
// output through ServerLogViewer — same pattern docker database backup
// runs use. NULL for historical/legacy jobs that predate task tracking.
func init() {
	Register(Migration{
		ID:        "0053_05_31_000000_add_task_id_to_backup_jobs",
		Name:      "Add task_id to backup_jobs",
		Timestamp: time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
		Up:        addTaskIDToBackupJobsUp,
		Down:      addTaskIDToBackupJobsDown,
	})
}

func addTaskIDToBackupJobsUp(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS task_id CHAR(26) NULL`).Error
}

func addTaskIDToBackupJobsDown(db *gorm.DB) error {
	return db.Exec(`ALTER TABLE backup_jobs DROP COLUMN IF EXISTS task_id`).Error
}
