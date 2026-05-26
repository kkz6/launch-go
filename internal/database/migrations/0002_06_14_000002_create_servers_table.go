package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000002_create_servers_table",
		Name:      "Create servers table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 2, 0, time.UTC),
		Up:        createServersTableUp,
		Down:      createServersTableDown,
	})
}

// serverMigration model for migration (matches Laravel schema)
type serverMigration struct {
	ID                        string     `gorm:"type:char(26);primaryKey"`
	ServerProviderID          *string    `gorm:"column:server_provider_id;type:char(26);index"`
	TeamID                    string     `gorm:"column:team_id;type:char(26);not null;index"`
	UserID                    string     `gorm:"column:user_id;type:char(26);not null;index"`
	Name                      string     `gorm:"type:varchar(255);not null;index"`
	Description               *string    `gorm:"type:varchar(255)"`
	Provider                  string     `gorm:"type:varchar(255);not null"`
	ProviderData              *string    `gorm:"type:jsonb"`
	Type                      *string    `gorm:"type:varchar(255)"`
	Connected                 bool       `gorm:"type:boolean;not null;default:0"`
	LaunchToken               string     `gorm:"type:varchar(32);not null"`
	MonitoringEnabled         bool       `gorm:"type:boolean;not null;default:0"`
	CPUCores                  *int       `gorm:"column:cpu_cores;type:int"`
	MemoryInMB                *int       `gorm:"column:memory_in_mb;type:int"`
	StorageInGB               *int       `gorm:"column:storage_in_gb;type:int"`
	OperatingSystem           *string    `gorm:"type:varchar(255)"`
	Status                    string     `gorm:"type:varchar(255);not null"`
	PublicIPv4                *string    `gorm:"column:public_ipv4;type:varchar(255)"`
	PrivateIPv4               *string    `gorm:"column:private_ipv4;type:varchar(255)"`
	PublicKey                 *string    `gorm:"type:text"`
	PrivateKey                *string    `gorm:"type:text"`
	UserPublicKey             *string    `gorm:"column:user_public_key;type:text"`
	Username                  *string    `gorm:"type:varchar(255)"`
	Password                  *string    `gorm:"type:text"`
	DatabasePassword          *string    `gorm:"type:text"`
	SSHPort                   *int       `gorm:"column:ssh_port;type:int"`
	WorkingDirectory          *string    `gorm:"type:varchar(255)"`
	CompletedProvisionSteps   *string    `gorm:"type:jsonb"`
	ProvisionedAt             *time.Time `gorm:"type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"type:timestamp null"`
	Updates                   bool       `gorm:"type:boolean;not null;default:0"`
	AutoUpdate                bool       `gorm:"type:boolean;not null;default:0"`
	AvailableUpdates          *int       `gorm:"type:int"`
	SecurityUpdates           *int       `gorm:"type:int"`
	Progress                  int        `gorm:"type:int;not null;default:0"`
	ProgressStep              *string    `gorm:"type:varchar(255)"`
	LastUpdateCheck           *time.Time `gorm:"type:timestamp null"`
	LastConnectivityCheck     *time.Time `gorm:"type:timestamp null"`
	ArchivedAt                *time.Time `gorm:"type:timestamp null"`
	CreatedAt                 *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                 *time.Time `gorm:"type:timestamp null"`
}

func (serverMigration) TableName() string {
	return "servers"
}

// serverWithFK defines foreign key relationships
type serverWithFK struct {
	ServerProviderID *string                  `gorm:"column:server_provider_id"`
	TeamID           string                   `gorm:"column:team_id"`
	UserID           string                   `gorm:"column:user_id"`
	ServerProvider   *serverProviderMigration `gorm:"foreignKey:ServerProviderID;references:ID;constraint:OnDelete:CASCADE"`
	Team             *teamMigration           `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
	User             *userMigration           `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (serverWithFK) TableName() string {
	return "servers"
}

// taskWithServerFK defines foreign key for tasks.server_id -> servers.id
type taskWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (taskWithServerFK) TableName() string {
	return "tasks"
}

func createServersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&serverMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for server_provider_id -> server_providers.id
	if err := migrator.CreateConstraint(&serverWithFK{}, "ServerProvider"); err != nil {
		return err
	}

	// Add foreign key constraint for team_id -> teams.id
	if err := migrator.CreateConstraint(&serverWithFK{}, "Team"); err != nil {
		return err
	}

	// Add foreign key constraint for user_id -> users.id
	if err := migrator.CreateConstraint(&serverWithFK{}, "User"); err != nil {
		return err
	}

	// Now add the FK constraint for tasks.server_id -> servers.id (deferred from tasks migration)
	return migrator.CreateConstraint(&taskWithServerFK{}, "Server")
}

func createServersTableDown(db *gorm.DB) error {
	migrator := db.Migrator()

	// Drop the tasks FK first
	_ = migrator.DropConstraint(&taskWithServerFK{}, "Server")

	return migrator.DropTable(&serverMigration{})
}
