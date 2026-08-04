package models

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// InstalledService represents an installed service on a server
type InstalledService struct {
	basemodels.BaseModel
	basemodels.ServerScoped
	Type     types.ServiceType   `gorm:"type:varchar(255);not null" json:"type"`
	TypeData dbtype.JSONMap      `gorm:"type:json" json:"-"`
	Name     string              `gorm:"type:varchar(255);not null" json:"name"`
	Version  string              `gorm:"type:varchar(255);not null" json:"version"`
	Status   types.ServiceStatus `gorm:"type:varchar(255);not null" json:"status"`
	// default:false is intentional — without it GORM relies on the
	// database column default, which was historically TRUE and caused
	// every newly-installed service to silently steal the "default"
	// flag from siblings (PHP 8.4 grabbing the star from PHP 8.3,
	// etc). Migration 0054 also flips the column default; the tag
	// makes the insert side belt-and-braces.
	IsDefault bool    `gorm:"column:is_default;type:tinyint(1);not null;default:false" json:"is_default"`
	Unit      *string `gorm:"type:varchar(255)" json:"unit,omitempty"`
	Software  string  `gorm:"type:varchar(255);not null" json:"software"`
	TaskID    *string `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
	Task   *Task   `gorm:"foreignKey:TaskID;references:ID" json:"task,omitempty"`
}

func (s *InstalledService) BeforeCreate(tx *gorm.DB) error {
	if err := s.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	basemodels.SetDefaultStatus(&s.Status, types.ServiceStatusPending)

	return nil
}

func (InstalledService) TableName() string {
	return "services"
}

func (s *InstalledService) GetFormattedVersion() string {
	if s.Version != "" {
		return s.Version
	}

	return ""
}

// GetServiceName returns the systemd service name for this installed service
func (s *InstalledService) GetServiceName() string {
	if s.Unit != nil && *s.Unit != "" {
		return *s.Unit
	}

	// Map service types to their typical systemd unit names
	switch s.Type {
	case types.ServiceTypeMySQL:
		return "mysql"
	case types.ServiceTypePostgreSQL:
		return "postgresql"
	case types.ServiceTypeRedis:
		return "redis-server"
	case types.ServiceTypeCaddy:
		return "caddy"
	case types.ServiceTypeSupervisor:
		return "supervisor"
	case types.ServiceTypePhp:
		return types.PhpFPMServiceFromVersion(s.PhpVersionSeries())
	case types.ServiceTypeLaunchAgent:
		return "launch-agent"
	default:
		return s.Name
	}
}

// GetSoftware returns the software enum for this service
func (s *InstalledService) GetSoftware() types.Software {
	return types.Software(s.Software)
}

// PhpVersionSeries returns the canonical major.minor PHP series. Software is
// the stable identity; Version may contain a detected patch such as 8.3.6.
func (s *InstalledService) PhpVersionSeries() string {
	software := s.GetSoftware()
	if software.IsPhp() {
		return software.GetVersion()
	}

	return types.PhpVersionSeries(s.Version)
}
