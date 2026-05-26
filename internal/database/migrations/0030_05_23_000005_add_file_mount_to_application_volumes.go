package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0030_05_23_000005_add_file_mount_to_application_volumes",
		Name:      "Add file-mount columns to docker_application_volumes",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 5, 0, time.UTC),
		Up:        addFileMountColumnsUp,
		Down:      addFileMountColumnsDown,
	})
}

// addFileMountColumnsUp lifts docker_application_volumes from a
// 2-flavour mount table ("named" + "bind") to dokploy's 3-flavour
// model:
//
//   - bind    → host_path:mount_path
//   - volume  → docker named volume :mount_path (formerly "named";
//     we accept both names on read for back-compat)
//   - file    → content is written to a file on the host at
//     file_path, then bind-mounted at mount_path. Useful
//     for config files (nginx.conf, .env) you want
//     versioned in the platform instead of baked into
//     the image.
//
// content is TEXT so we can carry whole config files;
// file_path keeps the host-side filename the deploy script writes
// to (defaults to the application's deploy directory).
func addFileMountColumnsUp(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes ADD COLUMN content TEXT NULL",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_volumes ADD COLUMN file_path VARCHAR(512) NULL",
	).Error
}

func addFileMountColumnsDown(db *gorm.DB) error {
	if err := db.Exec(
		"ALTER TABLE docker_application_volumes DROP COLUMN IF EXISTS file_path",
	).Error; err != nil {
		return err
	}
	return db.Exec(
		"ALTER TABLE docker_application_volumes DROP COLUMN IF EXISTS content",
	).Error
}
