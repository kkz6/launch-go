package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds GitHub-Actions build support to docker_applications. Two
// columns: build_location switches the source-type-handling between
// the on-server build path (default, today's behaviour) and the new
// "build via GitHub Actions, deploy resulting image" path; the token
// hash is a sha256 of a per-app deploy token used to authenticate
// inbound webhooks from the customer's GitHub Actions runs.
//
// The raw token value is shown to the user once at enable time and
// then never persisted — only its sha256 lives in this column. Hash
// compare uses subtle.ConstantTimeCompare in the handler so a leaked
// hash can't be brute-forced via timing.
//
// All other GHA config lives in the existing source_config JSON
// column; no new top-level columns needed for it.
func init() {
	Register(Migration{
		ID:        "0050_05_28_000002_add_build_location_to_docker_applications",
		Name:      "Add build_location + gha_deploy_token_hash to docker_applications",
		Timestamp: time.Date(2026, 5, 28, 0, 0, 2, 0, time.UTC),
		Up:        addBuildLocationToDockerApplicationsUp,
		Down:      addBuildLocationToDockerApplicationsDown,
	})
}

func addBuildLocationToDockerApplicationsUp(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE docker_applications
			ADD COLUMN build_location VARCHAR(32) NOT NULL DEFAULT 'server',
			ADD COLUMN gha_deploy_token_hash VARCHAR(255) NULL
	`).Error
}

func addBuildLocationToDockerApplicationsDown(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE docker_applications
			DROP COLUMN IF EXISTS build_location,
			DROP COLUMN IF EXISTS gha_deploy_token_hash
	`).Error
}
