package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0023_05_22_000002_add_internal_port_to_docker_applications",
		Name:      "Add internal_port to docker_applications",
		Timestamp: time.Date(2023, 5, 22, 0, 0, 2, 0, time.UTC),
		Up:        addInternalPortToDockerApplicationsUp,
	})
}

// addInternalPortToDockerApplicationsUp adds the port the container's
// service listens on inside the container. Traefik routes to this port
// over the launch-network, so deploys without a configured port can't be
// reached even after Traefik has the routing rule.
//
// Default 80 covers the common case (most app images expose 80); users
// can override it from the create form for non-default images (e.g. Node
// apps on 3000, Go services on 8080).
func addInternalPortToDockerApplicationsUp(db *gorm.DB) error {
	return db.Exec("ALTER TABLE docker_applications ADD COLUMN internal_port INT NOT NULL DEFAULT 80").Error
}
