package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ApplicationEnvVar is one key/value pair passed to the container via
// `docker run -e`. Secret flag controls whether the value is masked in
// list responses — same defence we use for database credentials.
//
// (application_id, key) is unique among live rows. Soft-delete is
// allowed so a removed env var doesn't block re-adding the same key
// later.
type ApplicationEnvVar struct {
	basemodels.BaseModel
	basemodels.SoftDeleteModel
	ApplicationID string `gorm:"column:application_id;type:char(26);not null;index" json:"application_id"`
	Key           string `gorm:"type:varchar(255);not null" json:"key"`
	Value         string `gorm:"type:longtext;not null" json:"-"`
	IsSecret      bool   `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
}

func (ApplicationEnvVar) TableName() string { return "docker_application_env_vars" }
