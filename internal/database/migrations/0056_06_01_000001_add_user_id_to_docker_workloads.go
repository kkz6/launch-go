package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Adds nullable `user_id` to docker_applications + docker_composes so
// every workload knows who created it. Needed for the GHA-permissions
// failure notification path — when the GitHub App's permissions get
// rejected at bootstrap time, we email the application's creator with
// instructions on how to update the App scopes.
//
// Nullable rather than NOT NULL so we don't have to fabricate values
// for the small number of legacy workloads created before this column
// existed. The CreateApplication / CreateCompose services stamp it on
// every new row from here on, so legacy = null, modern = a real
// users.id.
//
// Backfill: copy user_id from the source_controls row referenced via
// source_config->>'source_control_id'. That's the user who connected
// the GitHub App and is (overwhelmingly) the same person who created
// the workload, so this gives the notification path a usable
// recipient without us having to take a manual data-cleanup pass.
//
// The JSON ->> operator returns NULL when the key is missing or
// source_config itself is NULL, which is exactly what we want — the
// inner SELECT returns NULL, the UPDATE no-ops, and the column stays
// NULL for rows where no source-control row is referenced.
func init() {
	Register(Migration{
		ID:        "0056_06_01_000001_add_user_id_to_docker_workloads",
		Name:      "Add user_id to docker_applications + docker_composes",
		Timestamp: time.Date(2026, 6, 1, 0, 0, 1, 0, time.UTC),
		Up:        addUserIDToDockerWorkloadsUp,
		Down:      addUserIDToDockerWorkloadsDown,
	})
}

func addUserIDToDockerWorkloadsUp(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE docker_applications
			ADD COLUMN user_id CHAR(26) NULL;
		CREATE INDEX IF NOT EXISTS docker_applications_user_id_idx
			ON docker_applications (user_id);

		ALTER TABLE docker_composes
			ADD COLUMN user_id CHAR(26) NULL;
		CREATE INDEX IF NOT EXISTS docker_composes_user_id_idx
			ON docker_composes (user_id);
	`).Error; err != nil {
		return err
	}

	// Backfill from source_controls.user_id where the workload's
	// source_config references one. UPDATE … FROM … WHERE … is
	// Postgres-style and matches the rest of the migrations in this
	// directory.
	if err := db.Exec(`
		UPDATE docker_applications da
		   SET user_id = sc.user_id
		  FROM source_controls sc
		 WHERE da.user_id IS NULL
		   AND sc.id = NULLIF(da.source_config->>'source_control_id', '')
	`).Error; err != nil {
		return err
	}

	return db.Exec(`
		UPDATE docker_composes dc
		   SET user_id = sc.user_id
		  FROM source_controls sc
		 WHERE dc.user_id IS NULL
		   AND sc.id = NULLIF(dc.source_config->>'source_control_id', '')
	`).Error
}

func addUserIDToDockerWorkloadsDown(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE docker_applications
			DROP COLUMN IF EXISTS user_id;

		ALTER TABLE docker_composes
			DROP COLUMN IF EXISTS user_id;
	`).Error
}
