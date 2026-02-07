package models

import (
	"github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type DockerDomain struct {
	basemodels.BaseModel

	DockerServiceID string                `gorm:"column:docker_service_id;type:char(26);not null;index" json:"docker_service_id"`
	Host            string                `gorm:"type:varchar(255);not null" json:"host"`
	Path            string                `gorm:"type:varchar(255);not null;default:/" json:"path"`
	ContainerPort   int                   `gorm:"type:int;not null;default:80" json:"container_port"`
	HTTPS           bool                  `gorm:"type:boolean;not null;default:true" json:"https"`
	CertificateType types.CertificateType `gorm:"type:varchar(50);not null;default:letsencrypt" json:"certificate_type"`
	ForceSSL        bool                  `gorm:"type:boolean;not null;default:true" json:"force_ssl"`

	// Relations
	DockerService *DockerService `gorm:"foreignKey:DockerServiceID" json:"docker_service,omitempty"`
}

func (DockerDomain) TableName() string { return "docker_domains" }
