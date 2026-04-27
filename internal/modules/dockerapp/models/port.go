package models

import basemodels "github.com/kkz6/launch-go/internal/pkg/models"

// Port is a published-port mapping passed as `-p HostPort:ContainerPort/proto`.
// Most apps use Traefik labels (no published ports); ports here are for
// admin tools or non-HTTP services.
type Port struct {
	basemodels.BaseModel

	AppID         string `gorm:"column:app_id;type:char(26);not null;index" json:"app_id"`
	HostPort      int    `gorm:"column:host_port;type:int;not null" json:"host_port"`
	ContainerPort int    `gorm:"column:container_port;type:int;not null" json:"container_port"`
	// Protocol is "tcp" or "udp". Defaults to tcp when unset.
	Protocol string `gorm:"type:varchar(8);not null;default:tcp" json:"protocol"`
}

// TableName overrides the default GORM table name.
func (Port) TableName() string { return "docker_app_ports" }
