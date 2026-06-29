package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000010_create_script_executions_table",
		Name:      "Create script_executions table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 10, 0, time.UTC),
		Up:        createScriptExecutionsTableUp,
	})
}

// scriptExecutionMigration model for migration
type scriptExecutionMigration struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement"`
	ScriptID   string     `gorm:"column:script_id;type:char(26);not null;index"`
	ServerID   string     `gorm:"column:server_id;type:char(26);not null;index"`
	User       *string    `gorm:"type:varchar(255)"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null"`
	CreatedAt  *time.Time `gorm:"type:timestamp null"`
	UpdatedAt  *time.Time `gorm:"type:timestamp null"`
}

func (scriptExecutionMigration) TableName() string {
	return "script_executions"
}

// scriptExecutionWithScriptFK defines the script foreign key
type scriptExecutionWithScriptFK struct {
	ScriptID string           `gorm:"column:script_id"`
	Script   *scriptMigration `gorm:"foreignKey:ScriptID;references:ID;constraint:OnDelete:CASCADE"`
}

func (scriptExecutionWithScriptFK) TableName() string {
	return "script_executions"
}

// scriptExecutionWithServerFK defines the server foreign key
type scriptExecutionWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (scriptExecutionWithServerFK) TableName() string {
	return "script_executions"
}

func createScriptExecutionsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&scriptExecutionMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&scriptExecutionWithScriptFK{}, "Script"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&scriptExecutionWithServerFK{}, "Server")
}
