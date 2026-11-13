package models

import (
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// ServerProvider represents a connected cloud provider account
type ServerProvider struct {
	basemodels.BaseModel
	UserID      string                 `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID      *string                `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Profile     *string                `gorm:"type:varchar(255)" json:"profile,omitempty"`
	Provider    types.ServerProvider   `gorm:"type:varchar(255);not null" json:"provider"`
	Credentials dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
	Connected   bool                   `gorm:"type:tinyint(1);not null;default:1" json:"connected"`
}

func (ServerProvider) TableName() string {
	return "server_providers"
}

// CanProvision reports whether the connection can be used to create a server.
func (p *ServerProvider) CanProvision(teamID string, provider types.ServerProvider) bool {
	return p != nil && p.TeamID != nil && *p.TeamID == teamID && p.Provider == provider && p.Connected
}
