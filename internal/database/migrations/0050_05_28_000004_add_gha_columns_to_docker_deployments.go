package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds GHA-specific columns to docker_deployments so a GitHub Actions
// run becomes a first-class deployment record. Three columns:
//
//   - trigger_source: tags how the deploy was initiated. Pre-existing
//     deploys were implicitly "manual"; the default fills that in.
//   - gha_run_id: the GitHub Actions run id, used as the webhook
//     idempotency key. Two notifies for the same run (network retry,
//     workflow re-run with the same id) reuse the existing row rather
//     than spawning a duplicate.
//   - gha_run_url: deep link back to the Actions run page, surfaced
//     in the deployment list "via GitHub Actions" badge.
//
// The partial UNIQUE index on (target_type, target_id, gha_run_id)
// is what makes the idempotent upsert in the webhook handler safe.
// WHERE gha_run_id IS NOT NULL keeps non-GHA deployments (where the
// column stays NULL forever) out of the uniqueness scope so they
// don't collide on each other's NULLs.
func init() {
	Register(Migration{
		ID:        "0050_05_28_000004_add_gha_columns_to_docker_deployments",
		Name:      "Add trigger_source + gha_run_id + gha_run_url to docker_deployments",
		Timestamp: time.Date(2026, 5, 28, 0, 0, 4, 0, time.UTC),
		Up:        addGHAColumnsToDockerDeploymentsUp,
		Down:      addGHAColumnsToDockerDeploymentsDown,
	})
}

func addGHAColumnsToDockerDeploymentsUp(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE docker_deployments
			ADD COLUMN trigger_source VARCHAR(32) NOT NULL DEFAULT 'manual',
			ADD COLUMN gha_run_id VARCHAR(64) NULL,
			ADD COLUMN gha_run_url VARCHAR(512) NULL
	`).Error; err != nil {
		return err
	}
	return db.Exec(`
		CREATE UNIQUE INDEX idx_docker_deployments_gha_run_id
			ON docker_deployments (target_type, target_id, gha_run_id)
			WHERE gha_run_id IS NOT NULL
	`).Error
}

func addGHAColumnsToDockerDeploymentsDown(db *gorm.DB) error {
	if err := db.Exec(`DROP INDEX IF EXISTS idx_docker_deployments_gha_run_id`).Error; err != nil {
		return err
	}
	return db.Exec(`
		ALTER TABLE docker_deployments
			DROP COLUMN IF EXISTS trigger_source,
			DROP COLUMN IF EXISTS gha_run_id,
			DROP COLUMN IF EXISTS gha_run_url
	`).Error
}
