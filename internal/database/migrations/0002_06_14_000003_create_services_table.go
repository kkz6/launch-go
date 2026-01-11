package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000003_create_services_table",
		Name:      "Create services table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 3, 0, time.UTC),
		Up:        createServicesTableUp,
		Down:      createServicesTableDown,
	})
}

// serviceMigration model for migration (matches Laravel schema)
type serviceMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;index"`
	Type      string     `gorm:"type:varchar(255);not null"`
	TypeData  *string    `gorm:"type:json"`
	Name      string     `gorm:"type:varchar(255);not null"`
	Version   string     `gorm:"type:varchar(255);not null"`
	Status    string     `gorm:"type:varchar(255);not null"`
	IsDefault bool       `gorm:"type:tinyint(1);not null;default:1"`
	Unit      *string    `gorm:"type:varchar(255)"`
	Software  string     `gorm:"type:varchar(255);not null"`
	TaskID    *string    `gorm:"column:task_id;type:char(26);index"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (serviceMigration) TableName() string {
	return "services"
}

// serviceWithFK defines foreign key relationships
type serviceWithFK struct {
	ServerID string           `gorm:"column:server_id"`
	TaskID   *string          `gorm:"column:task_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
	Task     *taskMigration   `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:SET NULL"`
}

func (serviceWithFK) TableName() string {
	return "services"
}

func createServicesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&serviceMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for server_id -> servers.id
	if err := migrator.CreateConstraint(&serviceWithFK{}, "Server"); err != nil {
		return err
	}

	// Note: task_id FK is optional since tasks may be deleted
	// Laravel uses nullable foreignIdFor without constrained() for task_id

	return nil
}

func createServicesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&serviceMigration{})
}
