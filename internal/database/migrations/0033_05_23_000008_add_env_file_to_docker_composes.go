package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0033_05_23_000008_add_env_file_to_docker_composes",
		Name:      "Add env_file column to docker_composes",
		Timestamp: time.Date(2024, 5, 23, 0, 0, 8, 0, time.UTC),
		Up:        addEnvFileToComposesUp,
	})
}

// addEnvFileToComposesUp adds the env_file TEXT column the
// compose Environment subtab persists into.
//
// The body of this column is written to `${COMPOSE_DIR}/.env` on the
// server right before each `docker compose up`. Docker compose picks
// it up automatically for ${VAR} substitution inside the YAML AND
// passes the same keys/values into containers that declare them in
// their `environment:` blocks (without explicit values).
//
// The column is TEXT rather than a structured key/value table
// because a) compose's .env semantics are line-based with comments
// and blank-line tolerance — a structured table would lose that
// fidelity, b) the UI is a textarea editor not a row-grid, c) the
// total payload is bounded by docker's env-block limit so a single
// column is fine. NULL means "no env file" (the default, matches the
// pre-migration behavior).
func addEnvFileToComposesUp(db *gorm.DB) error {
	return db.Exec(
		"ALTER TABLE docker_composes ADD COLUMN env_file TEXT NULL",
	).Error
}
