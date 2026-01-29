package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_05_31_000000_create_source_controls_table",
		Name:      "Create source_controls table",
		Timestamp: time.Date(2003, 5, 31, 0, 0, 0, 0, time.UTC),
		Up:        createSourceControlsTableUp,
		Down:      createSourceControlsTableDown,
	})
}

// sourceControlMigration model for migration (matches Laravel schema with account fields merged)
type sourceControlMigration struct {
	ID                      string     `gorm:"type:char(26);primaryKey"`
	UserID                  string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID                  *string    `gorm:"column:team_id;type:char(26);index"`
	ProviderID              string     `gorm:"column:provider_id;type:varchar(255);not null;index"`
	ProviderAccountID       *string    `gorm:"column:provider_account_id;type:varchar(255)"`
	Login                   *string    `gorm:"type:varchar(255)"`
	Name                    *string    `gorm:"type:varchar(255)"`
	Type                    *string    `gorm:"type:varchar(255)"` // User, Organization, etc.
	AvatarURL               *string    `gorm:"column:avatar_url;type:varchar(255)"`
	HTMLURL                 *string    `gorm:"column:html_url;type:varchar(255)"`
	InstallationID          *string    `gorm:"column:installation_id;type:varchar(255)"`
	Permissions             *string    `gorm:"type:json"`
	RepositorySelection     *string    `gorm:"column:repository_selection;type:varchar(255)"`
	HasMultipleRepositories bool       `gorm:"column:has_multiple_repositories;default:false"`
	RepositoryCount         *int       `gorm:"column:repository_count"`
	ConnectedAt             *time.Time `gorm:"column:connected_at;type:timestamp null"`
	LastSyncedAt            *time.Time `gorm:"column:last_synced_at;type:timestamp null"`
	AdditionalData          *string    `gorm:"column:additional_data;type:json"`
	Provider                string     `gorm:"type:varchar(255);not null;index"` // github, gitlab, bitbucket
	URL                     *string    `gorm:"type:varchar(255)"`
	ProviderData            *string    `gorm:"column:provider_data;type:json"`
	TokenExpiresAt          *time.Time `gorm:"column:token_expires_at;type:timestamp null"`
	CreatedAt               *time.Time `gorm:"type:timestamp null"`
	UpdatedAt               *time.Time `gorm:"type:timestamp null"`
}

func (sourceControlMigration) TableName() string {
	return "source_controls"
}

// sourceControlWithFK defines foreign key relationships
type sourceControlWithFK struct {
	UserID string         `gorm:"column:user_id"`
	TeamID *string        `gorm:"column:team_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (sourceControlWithFK) TableName() string {
	return "source_controls"
}

func createSourceControlsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&sourceControlMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&sourceControlWithFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&sourceControlWithFK{}, "Team")
}

func createSourceControlsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&sourceControlMigration{})
}
