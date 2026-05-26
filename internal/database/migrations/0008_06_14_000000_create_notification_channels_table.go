package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0008_06_14_000000_create_notification_channels_table",
		Name:      "Create notification_channels table",
		Timestamp: time.Date(2008, 6, 14, 0, 0, 0, 0, time.UTC),
		Up:        createNotificationChannelsTableUp,
		Down:      createNotificationChannelsTableDown,
	})
}

// notificationChannelMigration model for migration
type notificationChannelMigration struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID    string     `gorm:"column:team_id;type:char(26);not null;index"`
	Provider  string     `gorm:"type:varchar(255);not null"`
	Label     string     `gorm:"type:varchar(255);not null"`
	Data      *string    `gorm:"type:jsonb"`
	Connected bool       `gorm:"default:false"`
	IsDefault bool       `gorm:"column:is_default;default:false"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (notificationChannelMigration) TableName() string {
	return "notification_channels"
}

// notificationChannelWithUserFK defines the user foreign key
type notificationChannelWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (notificationChannelWithUserFK) TableName() string {
	return "notification_channels"
}

// notificationChannelWithTeamFK defines the team foreign key
type notificationChannelWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (notificationChannelWithTeamFK) TableName() string {
	return "notification_channels"
}

func createNotificationChannelsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&notificationChannelMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&notificationChannelWithUserFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&notificationChannelWithTeamFK{}, "Team")
}

func createNotificationChannelsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&notificationChannelMigration{})
}
