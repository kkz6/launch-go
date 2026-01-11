package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// InstalledService represents an installed service on a server
type InstalledService struct {
	ID        string              `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string              `gorm:"size:26;not null;index" json:"server_id"`
	Type      enums.ServiceType   `gorm:"size:50;not null" json:"type"`
	TypeData  *string             `gorm:"type:json" json:"-"`
	Name      string              `gorm:"size:255;not null" json:"name"`
	Version   *string             `gorm:"size:50" json:"version,omitempty"`
	Status    enums.ServiceStatus `gorm:"size:50;default:'pending'" json:"status"`
	IsDefault bool                `gorm:"default:false" json:"is_default"`
	Unit      *string             `gorm:"size:255" json:"unit,omitempty"`
	Software  *enums.Software     `gorm:"size:50" json:"software,omitempty"`
	TaskID    *string             `gorm:"size:26" json:"task_id,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (s *InstalledService) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
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
	if s.Software != nil {
		return s.Software.GetVersion()
	}

	if s.Version != nil {
		return *s.Version
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
