package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0008_06_14_000001_create_monitor_notification_channel_table",
		Name:      "Create monitor_notification_channel pivot table",
		Timestamp: time.Date(2008, 6, 14, 0, 0, 1, 0, time.UTC),
		Up:        createMonitorNotificationChannelTableUp,
	})
}

// monitorNotificationChannelMigration model for pivot table
type monitorNotificationChannelMigration struct {
	MonitorID             uint64     `gorm:"column:monitor_id;not null;index"`
	NotificationChannelID uint64     `gorm:"column:notification_channel_id;not null;index"`
	CreatedAt             *time.Time `gorm:"type:timestamp null"`
	UpdatedAt             *time.Time `gorm:"type:timestamp null"`
}

func (monitorNotificationChannelMigration) TableName() string {
	return "monitor_notification_channel"
}

// monitorNotificationChannelWithMonitorFK defines the monitor foreign key
type monitorNotificationChannelWithMonitorFK struct {
	MonitorID uint64            `gorm:"column:monitor_id"`
	Monitor   *monitorMigration `gorm:"foreignKey:MonitorID;references:ID;constraint:OnDelete:CASCADE"`
}

func (monitorNotificationChannelWithMonitorFK) TableName() string {
	return "monitor_notification_channel"
}

// monitorNotificationChannelWithChannelFK defines the notification channel foreign key
type monitorNotificationChannelWithChannelFK struct {
	NotificationChannelID uint64                        `gorm:"column:notification_channel_id"`
	NotificationChannel   *notificationChannelMigration `gorm:"foreignKey:NotificationChannelID;references:ID;constraint:OnDelete:CASCADE"`
}

func (monitorNotificationChannelWithChannelFK) TableName() string {
	return "monitor_notification_channel"
}

func createMonitorNotificationChannelTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&monitorNotificationChannelMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&monitorNotificationChannelWithMonitorFK{}, "Monitor"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&monitorNotificationChannelWithChannelFK{}, "NotificationChannel")
}
