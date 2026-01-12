package models

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// InstalledService represents an installed service on a server
type InstalledService struct {
	basemodels.BaseModel
	ServerID  string              `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Type      enums.ServiceType   `gorm:"type:varchar(255);not null" json:"type"`
	TypeData  *string             `gorm:"type:json" json:"-"`
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

	if s.Status == "" {
		s.Status = enums.ServiceStatusPending
	}

	return nil
}

func (s *InstalledService) TableName() string {
	return "services"
}

func (s *InstalledService) GetFormattedVersion() string {
	if s.Version != "" {
		return s.Version
	}

	return ""
}

func (s *InstalledService) GetTypeData() map[string]interface{} {
	if s.TypeData == nil {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*s.TypeData), &data); err != nil {
		return nil
	}

	return data
}

func (s *InstalledService) SetTypeData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	str := string(jsonData)
	s.TypeData = &str

	return nil
}
