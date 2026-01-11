package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000005_create_crons_table",
		Name:      "Create crons table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 5, 0, time.UTC),
		Up:        createCronsTableUp,
		Down:      createCronsTableDown,
	})
}

// cronMigration model for migration (matches Laravel schema)
type cronMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	User                      string     `gorm:"type:varchar(255);not null"`
	Expression                string     `gorm:"type:varchar(255);not null"`
	Command                   string     `gorm:"type:longtext;not null"`
	Frequency                 string     `gorm:"type:varchar(255);not null"`
	Hidden                    bool       `gorm:"type:tinyint(1);not null;default:0"`
	InstalledAt               *time.Time `gorm:"type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (cronMigration) TableName() string {
	return "crons"
}

// cronWithFK defines foreign key relationships
type cronWithFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (cronWithFK) TableName() string {
	return "crons"
}

func createCronsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&cronMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&cronWithFK{}, "Server"); err != nil {
		return err
	}

	return nil
}

func createCronsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&cronMigration{})
}
