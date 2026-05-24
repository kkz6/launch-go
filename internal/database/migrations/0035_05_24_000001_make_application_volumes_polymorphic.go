package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID: "0035_05_24_000001_make_application_volumes_polymorphic",
		Name: "Make docker_application_volumes polymorphic " +
			"(application_id OR compose_id)",
		Timestamp: time.Date(2026, 5, 24, 0, 0, 1, 0, time.UTC),
		Up:        makeApplicationVolumesPolymorphicUp,
		Down:      makeApplicationVolumesPolymorphicDown,
	})
}

// makeApplicationVolumesPolymorphicUp turns docker_application_volumes
// into the shared docker mount table — same row backs application
// workloads AND compose stacks, with a nullable `application_id` and a
// new nullable `compose_id`. Exactly one is set per row (enforced at
// the service layer; we don't add a CHECK constraint because MySQL's
// support is uneven across the versions we target).
//
// Per-owner name uniqueness is preserved by adding a parallel unique
// index on (compose_id, name, deleted_at). The existing index on
// (application_id, name, deleted_at) keeps working — MySQL treats
// NULLs as distinct in UNIQUE indexes, so compose-only rows (with
// application_id NULL) don't collide.
//
// We deliberately do NOT rename the table. Renaming would force every
// model / repo / handler reference to shift in the same PR, which is
// noise — the type's GoDoc already explains the polymorphic shape.
func makeApplicationVolumesPolymorphicUp(db *gorm.DB) error {
	// 1. Relax application_id NOT NULL. Existing rows stay non-NULL so
	//    this is a strictly-widening change — no data migration needed.
	//    The original FK (docker_application_volumes.application_id →
	//    docker_applications.id ON DELETE CASCADE) is unaffected: FK
	//    constraints don't care about NOT NULL, they only enforce
	//    when the value is non-NULL.
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes " +
			"MODIFY COLUMN application_id CHAR(26) NULL",
	).Error; err != nil {
		return err
	}

	// 2. Add the compose owner column. Nullable so existing
	//    application-owned rows pass through unchanged.
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes " +
			"ADD COLUMN compose_id CHAR(26) NULL AFTER application_id",
	).Error; err != nil {
		return err
	}

	// 3. Lookup index for the compose List queries (mirrors the
	//    existing application_id index from the create migration).
	if err := db.Exec(
		"CREATE INDEX idx_docker_app_volume_compose_id " +
			"ON docker_application_volumes (compose_id)",
	).Error; err != nil {
		return err
	}

	// 4. Per-compose name uniqueness (live rows only — deleted_at
	//    is part of the index so soft-deletes don't block reuse).
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_docker_app_volume_compose_name " +
			"ON docker_application_volumes (compose_id, name, deleted_at)",
	).Error; err != nil {
		return err
	}

	// 5. FK from compose_id → docker_composes.id with ON DELETE
	//    CASCADE so removing a compose stack tears down its mount
	//    rows. Matches the existing application FK behavior.
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes " +
			"ADD CONSTRAINT fk_docker_app_volumes_compose " +
			"FOREIGN KEY (compose_id) REFERENCES docker_composes(id) " +
			"ON DELETE CASCADE",
	).Error; err != nil {
		return err
	}

	return nil
}

// makeApplicationVolumesPolymorphicDown reverses Up. Will fail if any
// compose-only rows exist (application_id IS NULL) — the rollback
// expects the caller to first migrate or delete those rows. That's
// intentional: silently dropping compose-owned mounts on a rollback
// would lose customer-defined state.
func makeApplicationVolumesPolymorphicDown(db *gorm.DB) error {
	// Order matters — drop the FK + indexes before the column itself.
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes " +
			"DROP FOREIGN KEY fk_docker_app_volumes_compose",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"DROP INDEX idx_docker_app_volume_compose_name " +
			"ON docker_application_volumes",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"DROP INDEX idx_docker_app_volume_compose_id " +
			"ON docker_application_volumes",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes DROP COLUMN compose_id",
	).Error; err != nil {
		return err
	}

	// Restore the NOT NULL constraint. This errors out if any row
	// snuck through with application_id NULL — surface that as a
	// hard failure rather than silent data loss.
	return db.Exec(
		"ALTER TABLE docker_application_volumes " +
			"MODIFY COLUMN application_id CHAR(26) NOT NULL",
	).Error
}
