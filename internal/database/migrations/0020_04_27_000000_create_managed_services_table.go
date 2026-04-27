package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0020_04_27_000000_create_managed_services_table",
		Name:      "Create managed services table",
		Timestamp: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		Up:        createManagedServicesTableUp,
		Down:      createManagedServicesTableDown,
	})
}

type managedServiceMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`

	Kind   string `gorm:"type:varchar(32);not null;index"`
	Status string `gorm:"type:varchar(32);not null;index"`

	Image     string `gorm:"type:varchar(255);not null"`
	Container string `gorm:"type:varchar(255);not null"`
	Volume    string `gorm:"type:varchar(255);not null"`

	DatabaseName *string `gorm:"column:database_name;type:varchar(255)"`
	Username     *string `gorm:"type:varchar(255)"`
	Password     string  `gorm:"type:varchar(255);not null"`

	LastError *string `gorm:"column:last_error;type:text"`
	TaskID    *string `gorm:"column:task_id;type:char(26);index"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (managedServiceMigration) TableName() string {
	return "managed_services"
}

type managedServiceWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (managedServiceWithTeamFK) TableName() string {
	return "managed_services"
}

type managedServiceWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (managedServiceWithServerFK) TableName() string {
	return "managed_services"
}

func createManagedServicesTableUp(db *gorm.DB) error {
	migrator := db.Set("gorm:table_options", "DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").Migrator()

	if err := migrator.CreateTable(&managedServiceMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&managedServiceWithTeamFK{}, "Team"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&managedServiceWithServerFK{}, "Server"); err != nil {
		return err
	}

	// One instance per (server, kind) — install rejects a second of the same kind.
	return db.Exec("CREATE UNIQUE INDEX idx_managed_services_server_kind ON managed_services(server_id, kind)").Error
}

func createManagedServicesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&managedServiceMigration{})
}
