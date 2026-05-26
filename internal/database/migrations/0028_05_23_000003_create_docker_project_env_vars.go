package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0028_05_23_000003_create_docker_project_env_vars",
		Name:      "Create docker_project_env_vars table",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 3, 0, time.UTC),
		Up:        createDockerProjectEnvVarsUp,
		Down:      createDockerProjectEnvVarsDown,
	})
}

// docker_project_env_vars — project-scoped key/value pairs that any
// container under the project can reference via ${{project.<KEY>}} in
// its own env-var values. Stored encrypted because they may carry
// shared secrets (DB connection strings, API keys) used by multiple
// workloads.
//
// Unique (project_id, key) among live rows so the resolver doesn't
// need to break ties. Soft-deletes allow re-adding the same key after
// removal.
type dockerProjectEnvVarMigration struct {
	ID        string         `gorm:"type:char(26);primaryKey"`
	ProjectID string         `gorm:"column:project_id;type:char(26);not null;index"`
	Key       string         `gorm:"type:varchar(255);not null"`
	Value     string         `gorm:"type:longtext;not null"`
	IsSecret  bool           `gorm:"column:is_secret;not null;default:false"`
	CreatedAt *time.Time     `gorm:"type:timestamp null"`
	UpdatedAt *time.Time     `gorm:"type:timestamp null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (dockerProjectEnvVarMigration) TableName() string {
	return "docker_project_env_vars"
}

type dockerProjectEnvVarWithProjectFK struct {
	ProjectID string                   `gorm:"column:project_id"`
	Project   *dockerProjectMigration  `gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerProjectEnvVarWithProjectFK) TableName() string {
	return "docker_project_env_vars"
}

func createDockerProjectEnvVarsUp(db *gorm.DB) error {
	migrator := db.Set(
		"gorm:table_options",
		"DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	).Migrator()

	if err := migrator.CreateTable(&dockerProjectEnvVarMigration{}); err != nil {
		return err
	}
	if err := migrator.CreateConstraint(&dockerProjectEnvVarWithProjectFK{}, "Project"); err != nil {
		return err
	}
	// Live-row uniqueness on (project_id, key). Including deleted_at in
	// the unique key lets soft-deleted rows coexist with a re-added one.
	return db.Exec(
		"CREATE UNIQUE INDEX idx_docker_project_env_vars_unique " +
			"ON docker_project_env_vars (project_id, `key`, deleted_at)",
	).Error
}

func createDockerProjectEnvVarsDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerProjectEnvVarMigration{})
}
