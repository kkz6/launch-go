package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0023_04_28_000000_add_compose_to_docker_apps",
		Name:      "Add compose source columns to docker_apps",
		Timestamp: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
		Up:        addComposeToDockerAppsUp,
		Down:      addComposeToDockerAppsDown,
	})
}

func addComposeToDockerAppsUp(db *gorm.DB) error {
	if err := db.Exec("ALTER TABLE docker_apps ADD COLUMN compose_yaml LONGTEXT NULL").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE docker_apps ADD COLUMN compose_env LONGTEXT NULL").Error; err != nil {
		return err
	}
	// image becomes nullable for compose-source apps.
	return db.Exec("ALTER TABLE docker_apps MODIFY COLUMN image VARCHAR(255) NULL").Error
}

func addComposeToDockerAppsDown(db *gorm.DB) error {
	if err := db.Exec("ALTER TABLE docker_apps MODIFY COLUMN image VARCHAR(255) NOT NULL").Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE docker_apps DROP COLUMN compose_env").Error; err != nil {
		return err
	}
	return db.Exec("ALTER TABLE docker_apps DROP COLUMN compose_yaml").Error
}
