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
	ID                        string           `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string           `gorm:"size:26;not null;index" json:"server_id"`
	Name                      string           `gorm:"size:255;not null" json:"name"`
	Action                    enums.RuleAction `gorm:"size:50;not null;default:'allow'" json:"action"`
	Port                      *string          `gorm:"size:50" json:"port,omitempty"`
	FromIPv4                  *string          `gorm:"size:45" json:"from_ipv4,omitempty"`
	Mask                      *string          `gorm:"size:10" json:"mask,omitempty"`
	Note                      *string          `gorm:"type:text" json:"note,omitempty"`
	InstalledAt               *time.Time       `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time       `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time       `json:"-"`
	UninstallationFailedAt    *time.Time       `json:"-"`
	CreatedAt                 time.Time        `json:"created_at"`
	UpdatedAt                 time.Time        `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
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

	if f.Port != nil && *f.Port != "" {
		parts = append(parts, *f.Port)
	}

	return strings.Join(parts, " ")
}
