package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000006_create_daemons_table",
		Name:      "Create daemons table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 6, 0, time.UTC),
		Up:        createDaemonsTableUp,
		Down:      createDaemonsTableDown,
	})
}

// daemonMigration model for migration (matches Laravel schema)
type daemonMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerID                  string     `gorm:"column:server_id;type:char(26);not null;index"`
	User                      string     `gorm:"type:varchar(255);not null"`
	Directory                 *string    `gorm:"type:varchar(255)"`
	Command                   string     `gorm:"type:longtext;not null"`
	Processes                 int        `gorm:"type:int;not null;default:1"`
	StopWaitSeconds           int        `gorm:"type:int;not null;default:10"`
	StopSignal                string     `gorm:"type:varchar(255);not null"`
	LastStatusCheck           *time.Time `gorm:"type:timestamp null"`
	Running                   bool       `gorm:"type:tinyint(1);not null;default:0"`
	Info                      *string    `gorm:"type:json"`
	InstalledAt               *time.Time `gorm:"type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (daemonMigration) TableName() string {
	return "daemons"
}

// daemonWithFK defines foreign key relationships
type daemonWithFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (daemonWithFK) TableName() string {
	return "daemons"
}

func createDaemonsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&daemonMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&daemonWithFK{}, "Server"); err != nil {
		return err
	}

	return nil
}

func createDaemonsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&daemonMigration{})
}
