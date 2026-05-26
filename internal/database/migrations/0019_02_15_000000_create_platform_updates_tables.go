package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0019_02_15_000000_create_platform_updates_tables",
		Name:      "Create platform_updates, server_platform_updates, and platform_update_dismissals tables",
		Timestamp: time.Date(2019, 2, 15, 0, 0, 0, 0, time.UTC),
		Up:        createPlatformUpdatesTablesUp,
		Down:      createPlatformUpdatesTablesDown,
	})
}

// platformUpdateMigration model for migration
type platformUpdateMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	Key         string     `gorm:"type:varchar(100);uniqueIndex;not null"`
	Title       string     `gorm:"type:varchar(255);not null"`
	Description string     `gorm:"type:text;not null"`
	Severity    string     `gorm:"type:varchar(50);not null;default:'info'"`
	ServerTypes *string    `gorm:"type:json"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (platformUpdateMigration) TableName() string {
	return "platform_updates"
}

// serverPlatformUpdateMigration model for migration
type serverPlatformUpdateMigration struct {
	ID               string     `gorm:"type:char(26);primaryKey"`
	ServerID         string     `gorm:"column:server_id;type:char(26);not null"`
	PlatformUpdateID string     `gorm:"column:platform_update_id;type:char(26);not null"`
	Status           string     `gorm:"type:varchar(50);not null;default:'pending'"`
	TaskID           *string    `gorm:"column:task_id;type:char(26)"`
	ErrorMessage     *string    `gorm:"column:error_message;type:text"`
	CompletedAt      *time.Time `gorm:"column:completed_at;type:timestamp null"`
	CreatedAt        *time.Time `gorm:"type:timestamp null"`
	UpdatedAt        *time.Time `gorm:"type:timestamp null"`
}

func (serverPlatformUpdateMigration) TableName() string {
	return "server_platform_updates"
}

// serverPlatformUpdateWithServerFK defines the server foreign key
type serverPlatformUpdateWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (serverPlatformUpdateWithServerFK) TableName() string {
	return "server_platform_updates"
}

// serverPlatformUpdateWithUpdateFK defines the platform update foreign key
type serverPlatformUpdateWithUpdateFK struct {
	PlatformUpdateID string                   `gorm:"column:platform_update_id"`
	PlatformUpdate   *platformUpdateMigration `gorm:"foreignKey:PlatformUpdateID;references:ID;constraint:OnDelete:CASCADE"`
}

func (serverPlatformUpdateWithUpdateFK) TableName() string {
	return "server_platform_updates"
}

// platformUpdateDismissalMigration model for migration
type platformUpdateDismissalMigration struct {
	ID               string     `gorm:"type:char(26);primaryKey"`
	UserID           string     `gorm:"column:user_id;type:char(26);not null"`
	PlatformUpdateID string     `gorm:"column:platform_update_id;type:char(26);not null"`
	CreatedAt        *time.Time `gorm:"type:timestamp null"`
}

func (platformUpdateDismissalMigration) TableName() string {
	return "platform_update_dismissals"
}

// platformUpdateDismissalWithUserFK defines the user foreign key
type platformUpdateDismissalWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (platformUpdateDismissalWithUserFK) TableName() string {
	return "platform_update_dismissals"
}

// platformUpdateDismissalWithUpdateFK defines the platform update foreign key
type platformUpdateDismissalWithUpdateFK struct {
	PlatformUpdateID string                   `gorm:"column:platform_update_id"`
	PlatformUpdate   *platformUpdateMigration `gorm:"foreignKey:PlatformUpdateID;references:ID;constraint:OnDelete:CASCADE"`
}

func (platformUpdateDismissalWithUpdateFK) TableName() string {
	return "platform_update_dismissals"
}

func createPlatformUpdatesTablesUp(db *gorm.DB) error {

	// Create platform_updates table
	if err := db.Migrator().CreateTable(&platformUpdateMigration{}); err != nil {
		return err
	}

	// Create server_platform_updates table
	if err := db.Migrator().CreateTable(&serverPlatformUpdateMigration{}); err != nil {
		return err
	}

	// Add unique index on (server_id, platform_update_id)
	if err := db.Exec("CREATE UNIQUE INDEX idx_server_platform_update ON server_platform_updates (server_id, platform_update_id)").Error; err != nil {
		return err
	}

	// Add foreign keys for server_platform_updates
	if err := db.Migrator().CreateConstraint(&serverPlatformUpdateWithServerFK{}, "Server"); err != nil {
		return err
	}

	if err := db.Migrator().CreateConstraint(&serverPlatformUpdateWithUpdateFK{}, "PlatformUpdate"); err != nil {
		return err
	}

	// Create platform_update_dismissals table
	if err := db.Migrator().CreateTable(&platformUpdateDismissalMigration{}); err != nil {
		return err
	}

	// Add unique index on (user_id, platform_update_id)
	if err := db.Exec("CREATE UNIQUE INDEX idx_user_platform_update_dismissal ON platform_update_dismissals (user_id, platform_update_id)").Error; err != nil {
		return err
	}

	// Add foreign keys for platform_update_dismissals
	if err := db.Migrator().CreateConstraint(&platformUpdateDismissalWithUserFK{}, "User"); err != nil {
		return err
	}

	return db.Migrator().CreateConstraint(&platformUpdateDismissalWithUpdateFK{}, "PlatformUpdate")
}

func createPlatformUpdatesTablesDown(db *gorm.DB) error {
	if err := db.Migrator().DropTable(&platformUpdateDismissalMigration{}); err != nil {
		return err
	}

	if err := db.Migrator().DropTable(&serverPlatformUpdateMigration{}); err != nil {
		return err
	}

	return db.Migrator().DropTable(&platformUpdateMigration{})
}
