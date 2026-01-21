package models

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// InstalledService represents an installed service on a server
type InstalledService struct {
	basemodels.BaseModel
	basemodels.ServerScopedModel
	Type      enums.ServiceType   `gorm:"type:varchar(255);not null" json:"type"`
	TypeData  basemodels.JSONMap  `gorm:"type:json" json:"-"`
	Name      string              `gorm:"type:varchar(255);not null" json:"name"`
	Version   string              `gorm:"type:varchar(255);not null" json:"version"`
	Status    enums.ServiceStatus `gorm:"type:varchar(255);not null" json:"status"`
	IsDefault bool                `gorm:"column:is_default;type:tinyint(1);not null;default:1" json:"is_default"`
	Unit      *string             `gorm:"type:varchar(255)" json:"unit,omitempty"`
	Software  string              `gorm:"type:varchar(255);not null" json:"software"`
	TaskID    *string             `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
	Task   *Task   `gorm:"foreignKey:TaskID;references:ID" json:"task,omitempty"`
}

func (s *InstalledService) BeforeCreate(tx *gorm.DB) error {
	if err := s.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	basemodels.SetDefaultStatus(&s.Status, enums.ServiceStatusPending)

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
	case enums.ServiceTypeMySql:
		return "mysql"
	case enums.ServiceTypePostgreSql:
		return "postgresql"
	case enums.ServiceTypeRedis:
		return "redis-server"
	case enums.ServiceTypeCaddy:
		return "caddy"
	case enums.ServiceTypeSupervisor:
		return "supervisor"
	case enums.ServiceTypePhp:
		return enums.PhpFpmServiceFromVersion(s.Version)
	case enums.ServiceTypeLaunchAgent:
		return "launch-agent"
	default:
		return s.Name
	}
}

// GetSoftware returns the software enum for this service
func (s *InstalledService) GetSoftware() enums.Software {
	return enums.Software(s.Software)
}
