package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0051_05_29_000000_create_docker_build_secrets",
		Name:      "Create docker_application_build_secrets and docker_compose_build_secrets tables",
		Timestamp: time.Date(2024, 5, 29, 0, 0, 0, 0, time.UTC),
		Up:        createDockerBuildSecretsUp,
	})
}

// docker_{application,compose}_build_secrets — values mounted into
// `docker build` via --mount=type=secret (BuildKit). Different from env
// vars in two ways:
//
//  1. They're available DURING build, not at runtime. A Dockerfile
//     uses them with `RUN --mount=type=secret,id=NAME cat /run/secrets/NAME ...`.
//  2. They're always secrets — there's no `is_secret` flag because
//     we never show the value back to the user. Reading requires
//     direct DB access (and goes through dbtype.EncryptedString).
//
// Stored encrypted at rest (same EncryptedString type as env vars).
// Soft-deleted; unique on (owner_id, name, deleted_at) so removing +
// re-adding the same name doesn't trip the index.
//
// For server-side builds the deploy task materialises each secret to
// a 0600 file under /run/launch/<deploy_id>.bsec/ (tmpfs) and passes
// `--secret id=NAME,src=...` to docker build. For GHA builds the
// bootstrap job pushes each value to GitHub as a repo secret named
// `LAUNCH_BUILD_<NAME>` and the workflow YAML references it under
// docker/build-push-action's `secrets:` input.
type dockerApplicationBuildSecretMigration struct {
	ID            string         `gorm:"type:char(26);primaryKey"`
	ApplicationID string         `gorm:"column:application_id;type:char(26);not null;index"`
	Name          string         `gorm:"type:varchar(255);not null"`
	Value         string         `gorm:"type:text;not null"`
	CreatedAt     *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt     *time.Time     `gorm:"type:timestamp null"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (dockerApplicationBuildSecretMigration) TableName() string {
	return "docker_application_build_secrets"
}

type dockerApplicationBuildSecretWithAppFK struct {
	ApplicationID string                      `gorm:"column:application_id"`
	Application   *dockerApplicationMigration `gorm:"foreignKey:ApplicationID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerApplicationBuildSecretWithAppFK) TableName() string {
	return "docker_application_build_secrets"
}

type dockerComposeBuildSecretMigration struct {
	ID        string         `gorm:"type:char(26);primaryKey"`
	ComposeID string         `gorm:"column:compose_id;type:char(26);not null;index"`
	Name      string         `gorm:"type:varchar(255);not null"`
	Value     string         `gorm:"type:text;not null"`
	CreatedAt *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt *time.Time     `gorm:"type:timestamp null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (dockerComposeBuildSecretMigration) TableName() string {
	return "docker_compose_build_secrets"
}

type dockerComposeBuildSecretWithComposeFK struct {
	ComposeID string                  `gorm:"column:compose_id"`
	Compose   *dockerComposeMigration `gorm:"foreignKey:ComposeID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerComposeBuildSecretWithComposeFK) TableName() string {
	return "docker_compose_build_secrets"
}

func createDockerBuildSecretsUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dockerApplicationBuildSecretMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerApplicationBuildSecretWithAppFK{}, "Application"); err != nil {
		return err
	}
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_docker_app_build_secrets_unique " +
			"ON docker_application_build_secrets (application_id, name, deleted_at)",
	).Error; err != nil {
		return err
	}

	if err := migrator.CreateTable(&dockerComposeBuildSecretMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerComposeBuildSecretWithComposeFK{}, "Compose"); err != nil {
		return err
	}
	return db.Exec(
		"CREATE UNIQUE INDEX idx_docker_compose_build_secrets_unique " +
			"ON docker_compose_build_secrets (compose_id, name, deleted_at)",
	).Error
}
