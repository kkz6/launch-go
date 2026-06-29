package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_05_31_000001_create_source_control_repositories_table",
		Name:      "Create source_control_repositories table",
		Timestamp: time.Date(2003, 5, 31, 0, 0, 1, 0, time.UTC),
		Up:        createSourceControlRepositoriesTableUp,
	})
}

// sourceControlRepositoryMigration model for migration
type sourceControlRepositoryMigration struct {
	ID              uint64  `gorm:"primaryKey;autoIncrement"`
	SourceControlID string  `gorm:"column:source_control_id;type:char(26);not null;index"`
	Name            string  `gorm:"type:varchar(255);not null"`
	FullName        string  `gorm:"column:full_name;type:varchar(255);not null"`
	Public          bool    `gorm:"default:false"`
	SSHURL          string  `gorm:"column:ssh_url;type:varchar(255);not null"`
	DefaultBranch   string  `gorm:"column:default_branch;type:varchar(255);not null"`
	HTMLURL         *string `gorm:"column:html_url;type:varchar(255)"`
	AdditionalData  *string `gorm:"column:additional_data;type:jsonb"`
}

func (sourceControlRepositoryMigration) TableName() string {
	return "source_control_repositories"
}

// sourceControlRepositoryWithFK defines the foreign key relationship
type sourceControlRepositoryWithFK struct {
	SourceControlID string                  `gorm:"column:source_control_id"`
	SourceControl   *sourceControlMigration `gorm:"foreignKey:SourceControlID;references:ID;constraint:OnDelete:CASCADE"`
}

func (sourceControlRepositoryWithFK) TableName() string {
	return "source_control_repositories"
}

func createSourceControlRepositoriesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&sourceControlRepositoryMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&sourceControlRepositoryWithFK{}, "SourceControl")
}
