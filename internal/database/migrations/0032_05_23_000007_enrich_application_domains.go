package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0032_05_23_000007_enrich_application_domains",
		Name:      "Add internal_path / strip_path / container_port / certificate_provider to docker_application_domains",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 7, 0, time.UTC),
		Up:        enrichApplicationDomainsUp,
		Down:      enrichApplicationDomainsDown,
	})
}

// enrichApplicationDomainsUp brings the docker domain row up to
// dokploy parity. New fields and why:
//
//   - internal_path        — the path the application expects
//                             internally. When path-external != internal,
//                             Traefik strips/rewrites accordingly.
//                             Defaults to "/" (matches dokploy).
//   - strip_path           — explicit toggle to strip the external
//                             `path` prefix before forwarding. Without
//                             this Traefik sends the full URL through;
//                             apps like Nuxt at /api want the prefix
//                             gone.
//   - container_port       — per-domain override of the application's
//                             internal_port. Useful when one container
//                             listens on multiple ports and different
//                             domains route to different ports.
//                             NULL = use the app's internal_port.
//   - certificate_provider — "letsencrypt" today; reserved for future
//                             support of ZeroSSL, Cloudflare Origin
//                             CA, etc. Default "letsencrypt".
func enrichApplicationDomainsUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_domains ADD COLUMN internal_path VARCHAR(255) NULL",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains ADD COLUMN strip_path BOOLEAN NOT NULL DEFAULT FALSE",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains ADD COLUMN container_port INT NULL",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_domains ADD COLUMN certificate_provider VARCHAR(32) NOT NULL DEFAULT 'letsencrypt'",
	).Error
}

func enrichApplicationDomainsDown(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN certificate_provider",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN container_port",
	).Error; err != nil {
		return err
	}
	if err := db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN strip_path",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_domains DROP COLUMN internal_path",
	).Error
}
