package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0018_02_10_000000_create_notification_preferences_table",
		Name:      "Create notification_preferences table",
		Timestamp: time.Date(2018, 2, 10, 0, 0, 0, 0, time.UTC),
		Up:        createNotificationPreferencesTableUp,
		Down:      createNotificationPreferencesTableDown,
	})
}

// notificationPreferenceMigration model for migration
type notificationPreferenceMigration struct {
	ID                     string     `gorm:"type:char(26);primaryKey"`
	TeamID                 string     `gorm:"column:team_id;type:char(26);not null;uniqueIndex"`
	EmailServerCreated     bool       `gorm:"column:email_server_created;default:true"`
	EmailServerDeleted     bool       `gorm:"column:email_server_deleted;default:true"`
	EmailDeploymentSuccess bool       `gorm:"column:email_deployment_success;default:false"`
	EmailDeploymentFailed  bool       `gorm:"column:email_deployment_failed;default:true"`
	EmailBackupSuccess     bool       `gorm:"column:email_backup_success;default:false"`
	EmailBackupFailed      bool       `gorm:"column:email_backup_failed;default:true"`
	CreatedAt              *time.Time `gorm:"type:timestamp null"`
	UpdatedAt              *time.Time `gorm:"type:timestamp null"`
}

func (notificationPreferenceMigration) TableName() string {
	return "notification_preferences"
}

// notificationPreferenceWithTeamFK defines the team foreign key
type notificationPreferenceWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (notificationPreferenceWithTeamFK) TableName() string {
	return "notification_preferences"
}

func createNotificationPreferencesTableUp(db *gorm.DB) error {
	if err := db.Migrator().CreateTable(&notificationPreferenceMigration{}); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&notificationPreferenceWithTeamFK{}, "Team")
}

func createNotificationPreferencesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&notificationPreferenceMigration{})
}
