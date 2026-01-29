package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0007_05_02_000001_create_domains_table",
		Name:      "Create domains table",
		Timestamp: time.Date(2007, 5, 2, 0, 0, 1, 0, time.UTC),
		Up:        createDomainsTableUp,
		Down:      createDomainsTableDown,
	})
}

// domainMigration model for migration
type domainMigration struct {
	ID               string     `gorm:"type:char(26);primaryKey"`
	UserID           string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID           *string    `gorm:"column:team_id;type:char(26);index"`
	DomainProviderID string     `gorm:"column:domain_provider_id;type:char(26);not null;index"`
	Label            string     `gorm:"type:varchar(255);not null"`
	Address          string     `gorm:"type:varchar(255);not null"`
	ProviderID       string     `gorm:"column:provider_id;type:varchar(255);not null"`
	AdditionalData   *string    `gorm:"column:additional_data;type:json"`
	CreatedAt        *time.Time `gorm:"type:timestamp null"`
	UpdatedAt        *time.Time `gorm:"type:timestamp null"`
}

func (domainMigration) TableName() string {
	return "domains"
}

// domainWithUserFK defines the user foreign key
type domainWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (domainWithUserFK) TableName() string {
	return "domains"
}

// domainWithTeamFK defines the team foreign key
type domainWithTeamFK struct {
	TeamID *string        `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (domainWithTeamFK) TableName() string {
	return "domains"
}

// domainWithDomainProviderFK defines the domain provider foreign key
type domainWithDomainProviderFK struct {
	DomainProviderID string                   `gorm:"column:domain_provider_id"`
	DomainProvider   *domainProviderMigration `gorm:"foreignKey:DomainProviderID;references:ID;constraint:OnDelete:CASCADE"`
}

func (domainWithDomainProviderFK) TableName() string {
	return "domains"
}

func createDomainsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&domainMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&domainWithUserFK{}, "User"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&domainWithTeamFK{}, "Team"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&domainWithDomainProviderFK{}, "DomainProvider")
}

func createDomainsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&domainMigration{})
}
