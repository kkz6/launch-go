package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// EnvVar is one VAR=value entry passed to the container at deploy time.
// The value is encrypted at rest because env vars routinely carry
// secrets (DB URLs, API tokens).
type EnvVar struct {
	basemodels.BaseModel

	AppID string `gorm:"column:app_id;type:char(26);not null;index" json:"app_id"`
	Key   string `gorm:"type:varchar(255);not null" json:"key"`
	// Value is encrypted at rest. UI never reads it back — edits replace
	// the value entirely.
	Value dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	// Secret marks values the UI should keep masked after save. The
	// stored value is encrypted regardless; this only changes display.
	Secret bool `gorm:"type:tinyint(1);not null;default:0" json:"secret"`
}

// TableName overrides the default GORM table name.
func (EnvVar) TableName() string { return "docker_app_env_vars" }
