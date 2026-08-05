package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0066_08_05_000000_add_site_id_to_tasks",
		Name:      "Add site_id to tasks",
		Timestamp: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		Up:        addSiteIDToTasksUp,
	})
}

// Tasks were server-scoped only, so a site-scoped task (switching a site's
// PHP runtime, for example) surfaced in Active Actions as a bare server
// action. The site was identifiable only from the task name text, which the
// UI cannot route on.
func addSiteIDToTasksUp(db *gorm.DB) error {
	statements := []string{
		"ALTER TABLE tasks ADD COLUMN IF NOT EXISTS site_id CHAR(26)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_site_id ON tasks (site_id)",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
