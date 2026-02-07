package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"

	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// LoadBalancerBackend links a specific site to a load balancer upstream as a backend
type LoadBalancerBackend struct {
	basemodels.BaseModel

	UpstreamID string `gorm:"size:26;not null;index" json:"upstream_id"`
	SiteID     string `gorm:"size:26;not null;index" json:"site_id"`
	ServerID   string `gorm:"size:26;not null;index" json:"server_id"`

	Port   int  `gorm:"default:8080" json:"port"`
	IsDown bool `gorm:"default:false" json:"is_down"`

	HealthStatus      types.HealthStatus `gorm:"column:health_status;size:50;default:unknown" json:"health_status"`
	LastHealthCheckAt *time.Time         `gorm:"column:last_health_check_at" json:"last_health_check_at,omitempty"`

	// Relations
	Upstream *LoadBalancerUpstream `gorm:"foreignKey:UpstreamID" json:"upstream,omitempty"`
	Server   *Server               `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (LoadBalancerBackend) TableName() string {
	return "load_balancer_backends"
}
