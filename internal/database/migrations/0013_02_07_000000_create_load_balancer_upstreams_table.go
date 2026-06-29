package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0013_02_07_000000_create_load_balancer_upstreams_table",
		Name:      "Create load balancer upstreams table",
		Timestamp: time.Date(2013, 2, 7, 0, 0, 0, 0, time.UTC),
		Up:        createLoadBalancerUpstreamsTableUp,
	})
}

type lbUpstreamMigration struct {
	ID       string `gorm:"type:char(26);primaryKey"`
	TeamID   string `gorm:"column:team_id;type:char(26);not null;index"`
	ServerID string `gorm:"column:server_id;type:char(26);not null;index"`

	Name    string `gorm:"type:varchar(255);not null"`
	Address string `gorm:"type:varchar(255);not null"`
	Port    int    `gorm:"type:int;default:443"`

	TLSSetting          string `gorm:"column:tls_setting;type:varchar(50);default:auto"`
	LBPolicy            string `gorm:"column:lb_policy;type:varchar(50);default:round_robin"`
	HealthCheckPath     string `gorm:"column:health_check_path;type:varchar(255);default:/health"`
	HealthCheckInterval string `gorm:"column:health_check_interval;type:varchar(20);default:30s"`
	HealthCheckTimeout  string `gorm:"column:health_check_timeout;type:varchar(20);default:10s"`

	InstalledAt               *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt      *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt    *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	PendingConfigUpdateSince  *time.Time `gorm:"column:pending_config_update_since;type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (lbUpstreamMigration) TableName() string {
	return "load_balancer_upstreams"
}

type lbUpstreamWithTeamFK struct {
	TeamID string         `gorm:"column:team_id"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (lbUpstreamWithTeamFK) TableName() string {
	return "load_balancer_upstreams"
}

type lbUpstreamWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (lbUpstreamWithServerFK) TableName() string {
	return "load_balancer_upstreams"
}

func createLoadBalancerUpstreamsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&lbUpstreamMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&lbUpstreamWithTeamFK{}, "Team"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&lbUpstreamWithServerFK{}, "Server"); err != nil {
		return err
	}

	return db.Exec("CREATE UNIQUE INDEX idx_lb_upstreams_server_address ON load_balancer_upstreams(server_id, address)").Error
}
