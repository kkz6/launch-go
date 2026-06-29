package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_01_19_000002_add_script_execution_columns",
		Name:      "Add columns to script_executions table",
		Timestamp: time.Date(2003, 1, 19, 0, 0, 2, 0, time.UTC),
		Up:        addScriptExecutionColumnsUp,
	})
}

func addScriptExecutionColumnsUp(db *gorm.DB) error {
	// Add batch_id column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN batch_id CHAR(26) NULL").Error; err != nil {
		return err
	}

	// Add status column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'pending'").Error; err != nil {
		return err
	}

	// Add exit_code column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN exit_code INT NULL").Error; err != nil {
		return err
	}

	// Add output column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN output TEXT NULL").Error; err != nil {
		return err
	}

	// Add started_at column
	if err := db.Exec("ALTER TABLE script_executions ADD COLUMN started_at TIMESTAMP NULL").Error; err != nil {
		return err
	}

	// Add index for batch_id
	if err := db.Exec("CREATE INDEX idx_script_executions_batch_id ON script_executions(batch_id)").Error; err != nil {
		return err
	}

	// Add index for status
	return db.Exec("CREATE INDEX idx_script_executions_status ON script_executions(status)").Error
}
