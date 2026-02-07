package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerEnvVar struct {
	basemodels.BaseModel

	DockerServiceID string `gorm:"column:docker_service_id;type:char(26);not null;index" json:"docker_service_id"`
	Key             string `gorm:"type:varchar(255);not null" json:"key"`
	Value           string `gorm:"type:text;not null" json:"value"`
	IsSecret        bool   `gorm:"type:boolean;not null;default:false" json:"is_secret"`
}

func (DockerEnvVar) TableName() string { return "docker_service_env_vars" }
