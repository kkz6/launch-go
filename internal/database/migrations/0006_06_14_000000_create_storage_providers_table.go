package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0006_06_14_000000_create_storage_providers_table",
		Name:      "Create storage_providers table",
		Timestamp: time.Date(2006, 6, 14, 0, 0, 0, 0, time.UTC),
		Up:        createStorageProvidersTableUp,
		Down:      createStorageProvidersTableDown,
	})
}

// storageProviderMigration model for migration
type storageProviderMigration struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement"`
	UserID         string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID         *string    `gorm:"column:team_id;type:char(26);index"`
	Provider       string     `gorm:"type:varchar(255);not null"`
	Label          *string    `gorm:"type:varchar(255)"`
	Token          *string    `gorm:"type:varchar(1000)"` // encrypted
	Credentials    *string    `gorm:"type:longtext"`      // encrypted
	RefreshToken   *string    `gorm:"column:refresh_token;type:varchar(1000)"`
	Connected      bool       `gorm:"default:true"`
	TokenExpiresAt *time.Time `gorm:"column:token_expires_at;type:timestamp null"`
	CreatedAt      *time.Time `gorm:"type:timestamp null"`
	UpdatedAt      *time.Time `gorm:"type:timestamp null"`
}

func (storageProviderMigration) TableName() string {
	return "storage_providers"
}

// storageProviderWithUserFK defines the user foreign key
type storageProviderWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (storageProviderWithUserFK) TableName() string {
	return "storage_providers"
}

// storageProviderWithTeamFK defines the team foreign key
type storageProviderWithTeamFK struct {
	TeamID *string        `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (storageProviderWithTeamFK) TableName() string {
	return "storage_providers"
}

func createStorageProvidersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&storageProviderMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&storageProviderWithUserFK{}, "User"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&storageProviderWithTeamFK{}, "Team"); err != nil {
		return err
	}

	return nil
}

func createStorageProvidersTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&storageProviderMigration{})
}
