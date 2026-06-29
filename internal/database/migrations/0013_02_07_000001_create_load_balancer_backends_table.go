package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0013_02_07_000001_create_load_balancer_backends_table",
		Name:      "Create load balancer backends table",
		Timestamp: time.Date(2013, 2, 7, 0, 0, 1, 0, time.UTC),
		Up:        createLoadBalancerBackendsTableUp,
	})
}

type lbBackendMigration struct {
	ID         string `gorm:"type:char(26);primaryKey"`
	UpstreamID string `gorm:"column:upstream_id;type:char(26);not null;index"`
	SiteID     string `gorm:"column:site_id;type:char(26);not null;index"`
	ServerID   string `gorm:"column:server_id;type:char(26);not null;index"`

	Port   int  `gorm:"type:int;default:8080"`
	IsDown bool `gorm:"column:is_down;default:false"`

	HealthStatus      string     `gorm:"column:health_status;type:varchar(50);default:unknown"`
	LastHealthCheckAt *time.Time `gorm:"column:last_health_check_at;type:timestamp null"`

	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (lbBackendMigration) TableName() string {
	return "load_balancer_backends"
}

type lbBackendWithUpstreamFK struct {
	UpstreamID string               `gorm:"column:upstream_id"`
	Upstream   *lbUpstreamMigration `gorm:"foreignKey:UpstreamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (lbBackendWithUpstreamFK) TableName() string {
	return "load_balancer_backends"
}

type lbBackendWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (lbBackendWithSiteFK) TableName() string {
	return "load_balancer_backends"
}

type lbBackendWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (lbBackendWithServerFK) TableName() string {
	return "load_balancer_backends"
}

func createLoadBalancerBackendsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&lbBackendMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&lbBackendWithUpstreamFK{}, "Upstream"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&lbBackendWithSiteFK{}, "Site"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&lbBackendWithServerFK{}, "Server"); err != nil {
		return err
	}

	return db.Exec("CREATE UNIQUE INDEX idx_lb_backends_upstream_site ON load_balancer_backends(upstream_id, site_id)").Error
}
