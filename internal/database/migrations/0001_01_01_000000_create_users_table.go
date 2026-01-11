package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0001_01_01_000000_create_users_table",
		Name:      "Create users table",
		Timestamp: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		Up:        createUsersTableUp,
		Down:      createUsersTableDown,
	})
}

// User model for migration (matches Laravel schema exactly)
type userMigration struct {
	ID                     string     `gorm:"type:char(26);primaryKey"`
	Name                   string     `gorm:"type:varchar(255);not null"`
	Email                  string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	EmailVerifiedAt        *time.Time `gorm:"type:timestamp null"`
	Password               string     `gorm:"type:varchar(255);not null"`
	TwoFactorSecret        *string    `gorm:"type:text"`
	TwoFactorRecoveryCodes *string    `gorm:"type:text"`
	TwoFactorConfirmedAt   *time.Time `gorm:"type:timestamp null"`
	RememberToken          *string    `gorm:"type:varchar(100)"`
	CurrentTeamID          *string    `gorm:"type:char(26)"`
	ProfilePhotoPath       *string    `gorm:"type:varchar(2048)"`
	Timezone               *string    `gorm:"type:varchar(255);default:'UTC'"`
	Onboarded              bool       `gorm:"type:tinyint(1);not null;default:0"`
	CreatedAt              *time.Time `gorm:"type:timestamp null"`
	UpdatedAt              *time.Time `gorm:"type:timestamp null"`
}

func (userMigration) TableName() string {
	return "users"
}

// PasswordResetToken model for migration
type passwordResetTokenMigration struct {
	Email     string     `gorm:"type:varchar(255);primaryKey"`
	Token     string     `gorm:"type:varchar(255);not null"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
}

func (passwordResetTokenMigration) TableName() string {
	return "password_reset_tokens"
}

// Session model for migration
type sessionMigration struct {
	ID           string  `gorm:"type:varchar(255);primaryKey"`
	UserID       *string `gorm:"type:char(26);index"`
	IPAddress    *string `gorm:"type:varchar(45)"`
	UserAgent    *string `gorm:"type:text"`
	Payload      string  `gorm:"type:longtext;not null"`
	LastActivity int     `gorm:"type:int;not null;index"`
}

func (sessionMigration) TableName() string {
	return "sessions"
}

func createUsersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	// Create users table
	if err := migrator.CreateTable(&userMigration{}); err != nil {
		return err
	}

	// Create password_reset_tokens table
	if err := migrator.CreateTable(&passwordResetTokenMigration{}); err != nil {
		return err
	}

	// Create sessions table
	if err := migrator.CreateTable(&sessionMigration{}); err != nil {
		return err
	}

	return nil
}

func createUsersTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.DropTable(&sessionMigration{}); err != nil {
		return err
	}
	if err := migrator.DropTable(&passwordResetTokenMigration{}); err != nil {
		return err
	}
	if err := migrator.DropTable(&userMigration{}); err != nil {
		return err
	}

	return nil
}
