package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationBuildSecret is a name/value pair mounted into `docker build`
// via BuildKit's --mount=type=secret. Distinct from ApplicationEnvVar in
// that it's NEVER passed to `docker run` — these are visible to the
// builder, not the running container — and the value is never returned
// to the UI.
//
// Identifiers follow the same POSIX env-name rules as env-var keys
// (the value is referenced by `id=NAME` from Dockerfiles, which uses
// the name like a shell variable). (application_id, name) is unique
// among live rows. Soft-delete is allowed so a removed secret doesn't
// block re-adding the same name later.
//
// Value is `dbtype.EncryptedString` — same at-rest encryption used for
// env-var values, database credentials, and storage-provider secrets.
type ApplicationBuildSecret struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string                 `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Name          string                 `gorm:"type:varchar(255);not null" json:"name"`
	Value         dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
}

func (ApplicationBuildSecret) TableName() string { return "docker_application_build_secrets" }

// ComposeBuildSecret is the compose-stack mirror of ApplicationBuildSecret.
// One stack publishes per-service images; a single build-secret name is
// available to every service that references it from its Dockerfile.
// Same encryption, uniqueness, and soft-delete semantics.
type ComposeBuildSecret struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ComposeID string                 `gorm:"column:compose_id;type:char(26);not null;index" json:"compose_id"`
	Name      string                 `gorm:"type:varchar(255);not null" json:"name"`
	Value     dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
}

func (ComposeBuildSecret) TableName() string { return "docker_compose_build_secrets" }
