package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationEnvVar is one key/value pair passed to the container via
// `docker run -e`. Secret flag controls whether the value is masked in
// list responses — same defence we use for database credentials.
//
// (application_id, key) is unique among live rows. Soft-delete is
// allowed so a removed env var doesn't block re-adding the same key
// later.
//
// Values may reference project env vars via `${{project.<KEY>}}`; the
// resolver in services/env_interpolation.go substitutes them at
// deploy/run time so a project-level change propagates on the next
// redeploy without rewriting per-container rows.
type ApplicationEnvVar struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string                 `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Key           string                 `gorm:"type:varchar(255);not null" json:"key"`
	Value         dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	IsSecret      bool                   `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
}

func (ApplicationEnvVar) TableName() string { return "docker_application_env_vars" }

// ProjectEnvVar is a project-scoped key/value pair. Workloads under
// the project reference it via `${{project.<KEY>}}` inside their own
// env vars. Shared destination for connection strings, API keys, and
// other secrets that multiple containers consume.
//
// Value is stored as `dbtype.EncryptedString` — the same at-rest
// encryption used for database credentials and storage-provider
// secrets. A leaked DB dump should never reveal raw env values.
//
// Same uniqueness + soft-delete semantics as ApplicationEnvVar, just
// scoped to project_id instead of application_id.
type ProjectEnvVar struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ProjectID string                 `gorm:"column:project_id;type:char(26);not null;index" json:"project_id"`
	Key       string                 `gorm:"type:varchar(255);not null" json:"key"`
	Value     dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	IsSecret  bool                   `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
}

func (ProjectEnvVar) TableName() string { return "docker_project_env_vars" }

// DatabaseEnvVar is one user-added env var on a managed database
// container. Sits alongside the auto-generated engine credentials
// (POSTGRES_USER etc) — both end up as `-e KEY=VALUE` arguments to
// the docker-run invocation. User vars come AFTER engine creds in
// the command, so a clashing key overrides the auto-cred (caller's
// responsibility — we don't block it).
//
// Value uses `dbtype.EncryptedString` — same at-rest encryption as
// ProjectEnvVar above; an env var carrying a password shouldn't sit
// in cleartext in the database.
//
// Same shape and uniqueness rules as ApplicationEnvVar / ProjectEnvVar,
// just keyed by database_id.
type DatabaseEnvVar struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	DatabaseID string                 `gorm:"column:database_id;type:char(26);not null;index" json:"database_id"`
	Key        string                 `gorm:"type:varchar(255);not null" json:"key"`
	Value      dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	IsSecret   bool                   `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
}

func (DatabaseEnvVar) TableName() string { return "docker_database_env_vars" }
