package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000004_create_queues_table",
		Name:      "Create queues table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 4, 0, time.UTC),
		Up:        createQueuesTableUp,
		Down:      createQueuesTableDown,
	})
}

// queueMigration model for migration
type queueMigration struct {
	ID                     string     `gorm:"type:char(26);primaryKey"`
	SiteID                 string     `gorm:"column:site_id;type:char(26);not null;index"`
	ServerID               string     `gorm:"column:server_id;type:char(26);not null;index"`
	UserID                 string     `gorm:"column:user_id;type:char(26);not null;index"`
	Directory              *string    `gorm:"type:varchar(255)"`
	Command                string     `gorm:"type:text;not null"`
	User                   string     `gorm:"type:varchar(255);not null"`
	AutoStart              bool       `gorm:"column:auto_start;default:true"`
	AutoRestart            bool       `gorm:"column:auto_restart;default:true"`
	Numprocs               int        `gorm:"default:8"`
	RedirectStderr         bool       `gorm:"column:redirect_stderr;default:true"`
	StopWaitSeconds        int        `gorm:"column:stop_wait_seconds;default:10"`
	StopSignal             string     `gorm:"column:stop_signal;type:varchar(255);not null"`
	QueueConnection        string     `gorm:"column:queue_connection;type:varchar(255);not null"`
	Environment            *string    `gorm:"type:varchar(255)"`
	Queue                  string     `gorm:"type:varchar(255);not null"`
	MaxSecondsPerJob       *int       `gorm:"column:max_seconds_per_job"`
	MaxTries               *int       `gorm:"column:max_tries"`
	RestSecondsOnEmpty     *int       `gorm:"column:rest_seconds_on_empty"`
	FailedJobDelaySeconds  *int       `gorm:"column:failed_job_delay_seconds"`
	MaxMemory              *int       `gorm:"column:max_memory"`
	RunOnMaintenance       bool       `gorm:"column:run_on_maintenance;default:false"`
	RunWithListen          bool       `gorm:"column:run_with_listen;default:false"`
	LastStatusCheck        *time.Time `gorm:"column:last_status_check;type:timestamp null"`
	Running                bool       `gorm:"default:false"`
	Info                   *string    `gorm:"type:json"`
	InstalledAt            *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt   *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	CreatedAt              *time.Time `gorm:"type:timestamp null"`
	UpdatedAt              *time.Time `gorm:"type:timestamp null"`
}

func (queueMigration) TableName() string {
	return "queues"
}

// queueWithSiteFK defines the site foreign key
type queueWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (queueWithSiteFK) TableName() string {
	return "queues"
}

// queueWithServerFK defines the server foreign key
type queueWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (queueWithServerFK) TableName() string {
	return "queues"
}

// queueWithUserFK defines the user foreign key
type queueWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (queueWithUserFK) TableName() string {
	return "queues"
}

func createQueuesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&queueMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&queueWithSiteFK{}, "Site"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&queueWithServerFK{}, "Server"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&queueWithUserFK{}, "User"); err != nil {
		return err
	}

	return nil
}

func createQueuesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&queueMigration{})
}
