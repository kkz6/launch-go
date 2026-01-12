package models

import (
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ServerProvider represents a connected cloud provider account
type ServerProvider struct {
	basemodels.BaseModel
	UserID      string               `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID      *string              `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Profile     *string              `gorm:"type:varchar(255)" json:"profile,omitempty"`
	Provider    enums.ServerProvider `gorm:"type:varchar(255);not null" json:"provider"`
	Credentials string               `gorm:"type:longtext;not null;serializer:encrypted" json:"-"`
	Connected   bool                 `gorm:"type:tinyint(1);not null;default:1" json:"connected"`
}

func (s *ServerProvider) TableName() string {
	return "server_providers"
}
