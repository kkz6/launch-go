package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0026_05_23_000001_add_build_config_to_docker_databases",
		Name:      "Add build_config to docker_databases",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 1, 0, time.UTC),
		Up:        addBuildConfigToDockerDatabasesUp,
		Down:      addBuildConfigToDockerDatabasesDown,
	})
}

// addBuildConfigToDockerDatabasesUp adds a build_config JSON column to
// docker_databases mirroring the same column on docker_applications.
// The Advanced subtab persists per-database runtime knobs here:
//
//   - restart_policy   (no/on-failure/always/unless-stopped)
//   - cpu_limit        (e.g. "0.5" / "2.0" → docker --cpus)
//   - memory_limit     (e.g. "512m" / "1g" → docker -m)
//   - cpu_reservation  (docker --cpu-shares analogue)
//   - memory_reservation (docker --memory-reservation)
//
// LONGTEXT keeps us future-proof for additional knobs without another
// migration — matches docker_applications.build_config.
func addBuildConfigToDockerDatabasesUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_databases ADD COLUMN build_config LONGTEXT NULL",
	).Error
}

func addBuildConfigToDockerDatabasesDown(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_databases DROP COLUMN build_config",
	).Error
}
