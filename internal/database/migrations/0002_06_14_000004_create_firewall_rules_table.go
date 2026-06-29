package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000004_create_firewall_rules_table",
		Name:      "Create firewall_rules table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 4, 0, time.UTC),
		Up:        createFirewallRulesTableUp,
	})
}

// firewallRuleMigration model for migration (matches Laravel schema)
type firewallRuleMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Name                      string     `gorm:"type:varchar(255);not null"`
	Action                    string     `gorm:"type:varchar(255);not null"`
	Port                      string     `gorm:"type:varchar(255);not null"`
	FromIPv4                  *string    `gorm:"column:from_ipv4;type:varchar(255)"`
	Mask                      *string    `gorm:"type:varchar(255)"`
	Note                      *string    `gorm:"type:text"`
	InstalledAt               *time.Time `gorm:"type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (firewallRuleMigration) TableName() string {
	return "firewall_rules"
}

// firewallRuleWithFK defines foreign key relationships
type firewallRuleWithFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (firewallRuleWithFK) TableName() string {
	return "firewall_rules"
}

func createFirewallRulesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&firewallRuleMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&firewallRuleWithFK{}, "Server")
}
