package models

import (
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// FirewallRule represents a firewall rule on a server
type FirewallRule struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	basemodels.ServerScoped
	Name     string           `gorm:"type:varchar(255);not null" json:"name"`
	Action   types.RuleAction `gorm:"type:varchar(255);not null" json:"action"`
	Port     string           `gorm:"type:varchar(255);not null" json:"port"`
	FromIPv4 *string          `gorm:"column:from_ipv4;type:varchar(255)" json:"from_ipv4,omitempty"`
	Mask     *string          `gorm:"type:varchar(255)" json:"mask,omitempty"`
	Note     *string          `gorm:"type:text" json:"note,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (f *FirewallRule) BeforeCreate(tx *gorm.DB) error {
	if err := f.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if f.Action == "" {
		f.Action = types.RuleActionAllow
	}

	return nil
}

func (FirewallRule) TableName() string {
	return "firewall_rules"
}

func (f *FirewallRule) IsPending() bool {
	return !f.IsInstalled() && !f.IsFailed()
}

func (f *FirewallRule) HasFailed() bool {
	return f.IsFailed()
}
