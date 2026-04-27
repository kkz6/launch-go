package models

import basemodels "github.com/kkz6/launch-go/internal/pkg/models"

// Domain describes one externally-routable domain pointing at the
// application via Traefik. Each domain row generates a router and a
// service in the container's traefik.* labels.
type Domain struct {
	basemodels.BaseModel

	AppID string `gorm:"column:app_id;type:char(26);not null;index" json:"app_id"`
	// Domain is the Host() rule, e.g. "api.example.com".
	Domain string `gorm:"type:varchar(255);not null;index" json:"domain"`
	// ContainerPort is the upstream port Traefik should send traffic to.
	// Required because containers can expose multiple ports.
	ContainerPort int `gorm:"column:container_port;type:int;not null" json:"container_port"`
	// TLS — when true, Traefik attaches the letsencrypt resolver. When
	// false the router only listens on :80.
	TLS bool `gorm:"type:tinyint(1);not null;default:1" json:"tls"`
}

// TableName overrides the default GORM table name.
func (Domain) TableName() string { return "docker_app_domains" }
