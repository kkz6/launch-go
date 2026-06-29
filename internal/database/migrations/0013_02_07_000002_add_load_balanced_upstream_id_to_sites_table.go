package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0013_02_07_000002_add_load_balanced_upstream_id_to_sites_table",
		Name:      "Add load_balanced_upstream_id to sites table",
		Timestamp: time.Date(2013, 2, 7, 0, 0, 2, 0, time.UTC),
		Up:        addLoadBalancedUpstreamIDToSitesUp,
	})
}

func addLoadBalancedUpstreamIDToSitesUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE sites ADD COLUMN load_balanced_upstream_id CHAR(26) NULL",
	).Error; err != nil {
		return err
	}

	if err := db.Exec(
		"CREATE INDEX idx_sites_lb_upstream ON sites(load_balanced_upstream_id)",
	).Error; err != nil {
		return err
	}

	return db.Exec(
		"ALTER TABLE sites ADD CONSTRAINT fk_sites_lb_upstream FOREIGN KEY (load_balanced_upstream_id) REFERENCES load_balancer_upstreams(id) ON DELETE SET NULL",
	).Error
}
