package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0034_05_24_000000_add_run_command_to_docker_composes",
		Name:      "Add run_command column to docker_composes",
		Timestamp: time.Date(2024, 5, 24, 0, 0, 0, 0, time.UTC),
		Up:        addRunCommandToComposesUp,
		Down:      addRunCommandToComposesDown,
	})
}

// addRunCommandToComposesUp adds the run_command TEXT column the
// compose Advanced subtab persists into.
//
// Stored as the body the user types — everything that comes AFTER
// docker in the deploy script. Defaults to nil; the deploy task
// branches on that:
//
//   - nil   → docker compose -p <name> -f <file> up -d --build --remove-orphans
//   - set   → docker <run_command>
//
// TEXT (rather than VARCHAR) because the user may chain
// `compose -p NAME -f FILE up -d --build --no-cache` plus extra
// service-name args or env-file overrides; the realistic ceiling is
// a few KB. NULL means "use the default", which is materially
// different from an empty string (= "run docker with no args"),
// hence nullable rather than NOT NULL with empty default.
func addRunCommandToComposesUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_composes ADD COLUMN run_command TEXT NULL",
	).Error
}

func addRunCommandToComposesDown(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_composes DROP COLUMN IF EXISTS run_command",
	).Error
}
