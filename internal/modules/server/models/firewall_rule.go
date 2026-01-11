package models

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// FirewallRule represents a firewall rule on a server
type FirewallRule struct {
	ID                        string           `gorm:"type:char(26);primaryKey" json:"id"`
	ServerID                  string           `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Name                      string           `gorm:"type:varchar(255);not null" json:"name"`
	Action                    enums.RuleAction `gorm:"type:varchar(255);not null" json:"action"`
	Port                      string           `gorm:"type:varchar(255);not null" json:"port"`
	FromIPv4                  *string          `gorm:"column:from_ipv4;type:varchar(255)" json:"from_ipv4,omitempty"`
	Mask                      *string          `gorm:"type:varchar(255)" json:"mask,omitempty"`
	Note                      *string          `gorm:"type:text" json:"note,omitempty"`
	InstalledAt               *time.Time       `gorm:"column:installed_at;type:timestamp null" json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time       `gorm:"column:installation_failed_at;type:timestamp null" json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time       `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	UninstallationFailedAt    *time.Time       `gorm:"column:uninstallation_failed_at;type:timestamp null" json:"-"`
	CreatedAt                 *time.Time       `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time       `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID;references:ID" json:"server,omitempty"`
}

func (f *FirewallRule) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = utils.NewULID()
	}

	if f.Action == "" {
		f.Action = enums.RuleActionAllow
	}

	return nil
}

func (f *FirewallRule) TableName() string {
	return "firewall_rules"
}

func (f *FirewallRule) IsInstalled() bool {
	return f.InstalledAt != nil
}

func (f *FirewallRule) IsPending() bool {
	return f.InstalledAt == nil && f.InstallationFailedAt == nil
}

func (f *FirewallRule) HasFailed() bool {
	return f.InstallationFailedAt != nil
}

func (f *FirewallRule) FormatAsUfwRule() string {
	parts := []string{f.Action.String()}

	if f.FromIPv4 != nil && *f.FromIPv4 != "" {
		parts = append(parts, fmt.Sprintf("from %s to any port", *f.FromIPv4))
	}

	if f.Port != "" {
		parts = append(parts, f.Port)
	}

	return strings.Join(parts, " ")
}
