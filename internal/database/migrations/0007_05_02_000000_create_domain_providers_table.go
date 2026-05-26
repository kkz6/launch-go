package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0007_05_02_000000_create_domain_providers_table",
		Name:      "Create domain_providers table",
		Timestamp: time.Date(2007, 5, 2, 0, 0, 0, 0, time.UTC),
		Up:        createDomainProvidersTableUp,
		Down:      createDomainProvidersTableDown,
	})
}

// domainProviderMigration model for migration (with sync_status fields merged)
type domainProviderMigration struct {
	ID               string     `gorm:"type:char(26);primaryKey"`
	UserID           string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID           *string    `gorm:"column:team_id;type:char(26);index"`
	Profile          *string    `gorm:"type:varchar(255)"`
	Provider         string     `gorm:"type:varchar(255);not null"`
	Credentials      string     `gorm:"type:text;not null"` // encrypted
	Connected        bool       `gorm:"default:true"`
	AdditionalData   *string    `gorm:"column:additional_data;type:json"`
	SyncStatus       string     `gorm:"column:sync_status;type:varchar(255);not null;default:idle"`
	LastSyncedAt     *time.Time `gorm:"column:last_synced_at;type:timestamp null"`
	SyncErrorMessage *string    `gorm:"column:sync_error_message;type:text"`
	CreatedAt        *time.Time `gorm:"type:timestamp null"`
	UpdatedAt        *time.Time `gorm:"type:timestamp null"`
}

func (domainProviderMigration) TableName() string {
	return "domain_providers"
}

// domainProviderWithUserFK defines the user foreign key
type domainProviderWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (domainProviderWithUserFK) TableName() string {
	return "domain_providers"
}

// domainProviderWithTeamFK defines the team foreign key
type domainProviderWithTeamFK struct {
	TeamID *string        `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (domainProviderWithTeamFK) TableName() string {
	return "domain_providers"
}

func createDomainProvidersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&domainProviderMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&domainProviderWithUserFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&domainProviderWithTeamFK{}, "Team")
}

func createDomainProvidersTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&domainProviderMigration{})
}
