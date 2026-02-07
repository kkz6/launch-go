package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerComposeProject struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped

	Name       string `gorm:"type:varchar(255);not null" json:"name"`
	RawCompose string `gorm:"type:text;not null" json:"raw_compose"`

	// Relations
	Services []DockerService `gorm:"foreignKey:ComposeProjectID" json:"services,omitempty"`
}

func (DockerComposeProject) TableName() string { return "docker_compose_projects" }
