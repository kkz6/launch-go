package models

import (
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerRegistry struct {
	basemodels.BaseModel
	basemodels.TeamScoped

	Name     string                 `gorm:"type:varchar(255);not null" json:"name"`
	URL      string                 `gorm:"type:varchar(500);not null" json:"url"`
	Username string                 `gorm:"type:varchar(255);not null" json:"username"`
	Password dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
}

func (DockerRegistry) TableName() string { return "docker_registries" }
