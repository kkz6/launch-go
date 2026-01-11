package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// ServerProvider represents a connected cloud provider account
type ServerProvider struct {
	ID          string               `gorm:"type:char(26);primaryKey" json:"id"`
	UserID      string               `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID      *string              `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Profile     *string              `gorm:"type:varchar(255)" json:"profile,omitempty"`
	Provider    enums.ServerProvider `gorm:"type:varchar(255);not null" json:"provider"`
	Credentials string               `gorm:"type:longtext;not null" json:"-"`
	Connected   bool                 `gorm:"type:tinyint(1);not null;default:1" json:"connected"`
	CreatedAt   *time.Time           `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt   *time.Time           `gorm:"type:timestamp null" json:"updated_at,omitempty"`
}

func (s *ServerProvider) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	return nil
}

func (s *ServerProvider) TableName() string {
	return "server_providers"
}
