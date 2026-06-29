package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0037_05_24_000003_create_registry_credentials",
		Name:      "Create registry_credentials + wire to docker apps + compose",
		Timestamp: time.Date(2026, 5, 24, 0, 0, 3, 0, time.UTC),
		Up:        createRegistryCredentialsUp,
	})
}

// createRegistryCredentialsUp lands the saved-credential table that
// backs docker image authentication. Same shape source_controls /
// storage_providers use — team-scoped (any team member can pick the
// credential when creating a workload), user-scoped (recorded for
// audit, no UI consequence today).
//
// Username + password are both encrypted at rest via the same
// dbtype.EncryptedString path env-var values use. Username gets
// encrypted too: it can leak account / registry-org names that are
// useful to an attacker even without the password, and the cost
// of encrypting a short string is nothing.
//
// Per-workload wiring:
//   - applications keep ONE credential (or inline creds), since one
//     image source needs at most one auth context. New columns on
//     docker_applications carry both the picked-cred ID and the
//     inline username/encrypted password.
//   - compose stacks attach 0..N credentials (a YAML can reference
//     services from multiple registries). The join table
//     docker_compose_registry_credentials carries the many-to-many.
func createRegistryCredentialsUp(db *gorm.DB) error {
	// 1. registry_credentials table
	if err := db.Exec(`
		CREATE TABLE registry_credentials (
			id            CHAR(26)     NOT NULL,
			team_id       CHAR(26)     NOT NULL,
			user_id       CHAR(26)     NULL,
			name          VARCHAR(255) NOT NULL,
			registry_url  VARCHAR(255) NULL,
			username      TEXT     NOT NULL,
			password      TEXT     NOT NULL,
			created_at    TIMESTAMP    NULL,
			updated_at    TIMESTAMP    NULL,
			deleted_at    TIMESTAMP    NULL,
			PRIMARY KEY (id),
			CONSTRAINT fk_reg_creds_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
		)
	`).Error; err != nil {
		return err
	}
	// Postgres requires CREATE INDEX statements outside the CREATE TABLE.
	if err := db.Exec(`CREATE INDEX idx_reg_creds_team ON registry_credentials (team_id)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX idx_reg_creds_deleted ON registry_credentials (deleted_at)`).Error; err != nil {
		return err
	}

	// 2. Per-team unique name (live rows only — deleted_at in the
	//    index so soft-deletes don't block re-using a name later).
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_reg_creds_team_name
		ON registry_credentials (team_id, name, deleted_at)
	`).Error; err != nil {
		return err
	}

	// 3. docker_applications gets three new columns — picked-cred ID
	//    plus the inline username/encrypted password. Service layer
	//    enforces at-most-one-of (saved vs inline).
	if err := db.Exec(`
		ALTER TABLE docker_applications
			ADD COLUMN registry_credential_id CHAR(26)     NULL,
			ADD COLUMN registry_username      VARCHAR(255) NULL,
			ADD COLUMN registry_password      TEXT         NULL
	`).Error; err != nil {
		return err
	}
	// Postgres requires CREATE INDEX as a separate statement.
	if err := db.Exec(`CREATE INDEX idx_docker_apps_reg_cred ON docker_applications (registry_credential_id)`).Error; err != nil {
		return err
	}

	// 4. FK from docker_applications.registry_credential_id to
	//    registry_credentials.id. ON DELETE SET NULL — deleting the
	//    saved credential shouldn't cascade-delete the application;
	//    we just disconnect it and the next deploy falls back to
	//    "no auth" (or fails on pull, which the operator can fix by
	//    re-attaching).
	if err := db.Exec(`
		ALTER TABLE docker_applications
			ADD CONSTRAINT fk_docker_apps_reg_cred
			FOREIGN KEY (registry_credential_id)
			REFERENCES registry_credentials(id)
			ON DELETE SET NULL
	`).Error; err != nil {
		return err
	}

	// 5. Many-to-many join for compose stacks. Composite PK keeps
	//    duplicates out at the DB level. Both FKs cascade so removing
	//    either side wipes the join row.
	if err := db.Exec(`
		CREATE TABLE docker_compose_registry_credentials (
			compose_id             CHAR(26) NOT NULL,
			registry_credential_id CHAR(26) NOT NULL,
			PRIMARY KEY (compose_id, registry_credential_id),
			CONSTRAINT fk_dcrc_compose
				FOREIGN KEY (compose_id) REFERENCES docker_composes(id) ON DELETE CASCADE,
			CONSTRAINT fk_dcrc_credential
				FOREIGN KEY (registry_credential_id) REFERENCES registry_credentials(id) ON DELETE CASCADE
		)
	`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX idx_dcrc_credential ON docker_compose_registry_credentials (registry_credential_id)`).Error
}
