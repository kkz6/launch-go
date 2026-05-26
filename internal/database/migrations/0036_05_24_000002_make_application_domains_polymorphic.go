package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID: "0036_05_24_000002_make_application_domains_polymorphic",
		Name: "Make docker_application_domains polymorphic " +
			"(application_id OR compose_id) + add service_name",
		Timestamp: time.Date(2026, 5, 24, 0, 0, 2, 0, time.UTC),
		Up:        makeApplicationDomainsPolymorphicUp,
		Down:      makeApplicationDomainsPolymorphicDown,
	})
}

// makeApplicationDomainsPolymorphicUp turns docker_application_domains
// into the shared workload-domain table. Same pattern as migration
// 0035 (volumes): application_id is relaxed to nullable, a compose_id
// column is added, and per-owner uniqueness is enforced via a parallel
// unique index. service_name is a new column that's only meaningful
// on compose-owned rows — it names the YAML service the domain
// routes to, since one compose stack hosts N services and the
// Traefik renderer needs to know which one each domain targets.
//
// FK constraints: application_id keeps its existing FK (which already
// tolerates NULL — FKs only enforce when the value is non-NULL).
// compose_id gets a new FK to docker_composes(id) ON DELETE CASCADE
// so removing a compose stack tears down its domain rows alongside
// the volumes that migration 0035 just landed.
//
// Table name `docker_application_domains` is preserved. Renaming it
// to `docker_workload_domains` would be a strictly cosmetic improve-
// ment that ripples through every reference; not worth the diff.
func makeApplicationDomainsPolymorphicUp(db *gorm.DB) error {
	// 1. Relax application_id NOT NULL. Existing rows stay non-NULL
	//    so this is strictly widening — no data migration needed.
	if err := db.Exec(
		"ALTER TABLE docker_application_domains " +
			"MODIFY COLUMN application_id CHAR(26) NULL",
	).Error; err != nil {
		return err
	}

	// 2. Add the compose owner column. Nullable so existing
	//    application-owned rows pass through unchanged.
	if err := db.Exec(
		"ALTER TABLE docker_application_domains " +
			"ADD COLUMN compose_id CHAR(26) NULL AFTER application_id",
	).Error; err != nil {
		return err
	}

	// 3. service_name — required on compose rows, NULL on application
	//    rows. We keep the column nullable rather than DEFAULT '' so
	//    the (compose_id, service_name) ↔ (application_id, NULL)
	//    distinction is unambiguous at the row level for diagnostics.
	if err := db.Exec(
		"ALTER TABLE docker_application_domains " +
			"ADD COLUMN service_name VARCHAR(255) NULL AFTER compose_id",
	).Error; err != nil {
		return err
	}

	// 4. Lookup index for compose List queries (mirrors the existing
	//    application_id index from the create migration).
	if err := db.Exec(
		"CREATE INDEX idx_docker_app_domain_compose_id " +
			"ON docker_application_domains (compose_id)",
	).Error; err != nil {
		return err
	}

	// 5. Per-compose host uniqueness (live rows only — deleted_at in
	//    the index so soft-deletes don't block reuse). MySQL treats
	//    NULL as distinct in UNIQUE indexes, so application-only
	//    rows (compose_id IS NULL) don't collide.
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_docker_app_domain_compose_host " +
			"ON docker_application_domains (compose_id, host, deleted_at)",
	).Error; err != nil {
		return err
	}

	// 6. FK from compose_id → docker_composes.id with ON DELETE
	//    CASCADE so removing a compose stack tears down its domain
	//    rows. Matches the existing application FK behavior.
	if err := db.Exec(
		"ALTER TABLE docker_application_domains " +
			"ADD CONSTRAINT fk_docker_app_domains_compose " +
			"FOREIGN KEY (compose_id) REFERENCES docker_composes(id) " +
			"ON DELETE CASCADE",
	).Error; err != nil {
		return err
	}

	return nil
}

// makeApplicationDomainsPolymorphicDown reverses Up. Will fail if
// compose-only rows exist (application_id IS NULL) — same intentional
// hard failure 0035 uses to prevent silent data loss on rollback.
func makeApplicationDomainsPolymorphicDown(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_domains " +
			"DROP FOREIGN KEY fk_docker_app_domains_compose",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"DROP INDEX idx_docker_app_domain_compose_host " +
			"ON docker_application_domains",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"DROP INDEX idx_docker_app_domain_compose_id " +
			"ON docker_application_domains",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN service_name",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN compose_id",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_domains " +
			"MODIFY COLUMN application_id CHAR(26) NOT NULL",
	).Error
}
