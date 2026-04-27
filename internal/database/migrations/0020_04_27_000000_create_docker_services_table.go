package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0020_04_27_000000_create_docker_services_table",
		Name:      "Create docker services table",
		Timestamp: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		Up:        createDockerServicesTableUp,
		Down:      createDockerServicesTableDown,
	})
}

type dockerServiceMigration struct {
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

func (dockerServiceMigration) TableName() string {
	return "docker_services"
}

type dockerServiceWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceWithTeamFK) TableName() string {
	return "docker_services"
}

type dockerServiceWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dockerServiceWithServerFK) TableName() string {
	return "docker_services"
}

func createDockerServicesTableUp(db *gorm.DB) error {
	migrator := db.Set("gorm:table_options", "DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").Migrator()

	if err := migrator.CreateTable(&dockerServiceMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithTeamFK{}, "Team"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dockerServiceWithServerFK{}, "Server"); err != nil {
		return err
	}

	// One instance per (server, kind) — install rejects a second of the same kind.
	return db.Exec("CREATE UNIQUE INDEX idx_docker_services_server_kind ON docker_services(server_id, kind)").Error
}

func createDockerServicesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dockerServiceMigration{})
}
