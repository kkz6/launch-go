package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Mirror of 0050_05_28_000002 for docker_composes. Compose stacks
// can build via GitHub Actions too; the workflow YAML for a compose
// row is structurally different (matrix per service that declares a
// `build:` directive) but the build_location switch + token hash are
// identical in shape. Keeping the two migrations separate (rather
// than one combined ALTER) so each workload's history is independent
// in case we ever want to roll back GHA on applications without
// touching composes.
func init() {
	Register(Migration{
		ID:        "0050_05_28_000003_add_build_location_to_docker_composes",
		Name:      "Add build_location + gha_deploy_token_hash to docker_composes",
		Timestamp: time.Date(2026, 5, 28, 0, 0, 3, 0, time.UTC),
		Up:        addBuildLocationToDockerComposesUp,
		Down:      addBuildLocationToDockerComposesDown,
	})
}

func addBuildLocationToDockerComposesUp(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE docker_composes
			ADD COLUMN build_location VARCHAR(32) NOT NULL DEFAULT 'server',
			ADD COLUMN gha_deploy_token_hash VARCHAR(255) NULL
	`).Error
}

func addBuildLocationToDockerComposesDown(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE docker_composes
			DROP COLUMN IF EXISTS build_location,
			DROP COLUMN IF EXISTS gha_deploy_token_hash
	`).Error
}
