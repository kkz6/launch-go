package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0025_05_23_000000_add_action_task_id_to_docker_deployments",
		Name:      "Add action + task_id columns to docker_deployments",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 0, 0, time.UTC),
		Up:        addActionTaskIDToDockerDeploymentsUp,
	})
}

// docker_deployments is polymorphic across applications + composes via
// (target_type, target_id). We're extending it to cover databases too —
// "database" becomes a valid target_type with action = create / start /
// restart / stop / rm. Apps + composes keep their existing rows
// untouched (their implicit action is "deploy").
//
// task_id binds the deployment row to the underlying taskrunner.Task
// (server-tasks table). Same pattern site deployments use — once
// persisted, the frontend can subscribe to the task entity logs
// websocket and stream the SSH output live (no need for a parallel
// log_path column or polling).
//
// Both columns are nullable: backfilling action onto historical rows
// would be guesswork, and task_id only exists once a worker dispatches
// the task. Frontend renders "—" / hides the View Logs button when
// missing.
func addActionTaskIDToDockerDeploymentsUp(db *gorm.DB) error {
	// Use raw SQL: gorm's AddColumn picks up tag defaults inconsistently
	// across MySQL versions when the column is nullable, and we want
	// explicit control over both nullability and indexes.
	if err := db.Exec(
		"ALTER TABLE docker_deployments " +
			"ADD COLUMN action VARCHAR(32) NULL, " +
			"ADD COLUMN task_id CHAR(26) NULL",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"CREATE INDEX idx_docker_deployments_action ON docker_deployments (action)",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"CREATE INDEX idx_docker_deployments_task_id ON docker_deployments (task_id)",
	).Error
}
