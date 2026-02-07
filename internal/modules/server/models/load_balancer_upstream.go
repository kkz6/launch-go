package models

import (
	"time"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"

	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// LoadBalancerUpstream represents a load balanced endpoint configuration on a load balancer server
type LoadBalancerUpstream struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.InstallableModel

	Name    string `gorm:"size:255;not null" json:"name"`
	Address string `gorm:"size:255;not null" json:"address"`
	Port    int    `gorm:"default:443" json:"port"`

	TLSSetting          string         `gorm:"column:tls_setting;size:50;default:auto" json:"tls_setting"`
	LBPolicy            types.LBPolicy `gorm:"column:lb_policy;size:50;default:round_robin" json:"lb_policy"`
	HealthCheckPath     string         `gorm:"size:255;default:/health" json:"health_check_path"`
	HealthCheckInterval string         `gorm:"size:20;default:30s" json:"health_check_interval"`
	HealthCheckTimeout  string         `gorm:"size:20;default:10s" json:"health_check_timeout"`

	PendingConfigUpdateSince *time.Time `gorm:"column:pending_config_update_since" json:"pending_config_update_since,omitempty"`

	// Relations
	Backends []LoadBalancerBackend `gorm:"foreignKey:UpstreamID" json:"backends,omitempty"`
	Server   *Server               `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (LoadBalancerUpstream) TableName() string {
	return "load_balancer_upstreams"
}

// LBPolicyLabel returns the display label for the load balancing policy
func (u *LoadBalancerUpstream) LBPolicyLabel() string {
	return u.LBPolicy.Label()
}
