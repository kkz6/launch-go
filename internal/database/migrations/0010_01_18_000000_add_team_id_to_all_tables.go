package migrations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0010_01_18_000000_add_team_id_to_all_tables",
		Name:      "Add team_id to all tables",
		Timestamp: time.Date(2010, 1, 18, 0, 0, 0, 0, time.UTC),
		Up:        addTeamIdToAllTablesUp,
		Down:      addTeamIdToAllTablesDown,
	})
}

// tableConfig defines how to add team_id to each table
type tableConfig struct {
	table       string
	afterColumn string
	parentTable string
	parentFK    string
}

// Tables that need team_id added (in dependency order)
// sites gets team_id from servers (via server_id)
// Deployments get team context through site relationship (site.team_id)
var tablesToUpdate = []tableConfig{
	// Sites first (depends on servers which already has team_id)
	{"sites", "server_id", "servers", "server_id"},
	// Then tables that depend on servers
	{"databases", "server_id", "servers", "server_id"},
	{"backups", "server_id", "servers", "server_id"},
	{"database_users", "server_id", "servers", "server_id"},
	// Then tables that depend on sites
	{"certificates", "site_id", "sites", "site_id"},
	{"commands", "site_id", "sites", "site_id"},
	{"queues", "site_id", "sites", "site_id"},
	{"redirects", "site_id", "sites", "site_id"},
	// Then tables that depend on backups
	{"backup_jobs", "backup_id", "backups", "backup_id"},
	// Then tables that depend on domains
	{"dns_records", "domain_id", "domains", "domain_id"},
}

// Tables with existing nullable team_id that need NOT NULL
var existingNullableTables = []string{
	"source_controls",
	"domains",
	"domain_providers",
	"storage_providers",
}

func addTeamIdToAllTablesUp(db *gorm.DB) error {
	// Add team_id to tables that don't have it
	for _, cfg := range tablesToUpdate {
		if err := addTeamIdColumn(db, cfg); err != nil {
			return fmt.Errorf("failed to add team_id to %s: %w", cfg.table, err)
		}
	}

	// Make existing nullable team_id columns NOT NULL
	for _, table := range existingNullableTables {
		if err := db.Exec(fmt.Sprintf(
			"ALTER TABLE `%s` MODIFY COLUMN team_id CHAR(26) NOT NULL", table,
		)).Error; err != nil {
			return fmt.Errorf("failed to make team_id NOT NULL on %s: %w", table, err)
		}
	}

	return nil
}

func addTeamIdColumn(db *gorm.DB, cfg tableConfig) error {
	// Add nullable column (backticks for reserved words like 'databases')
	if err := db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s` ADD COLUMN team_id CHAR(26) NULL AFTER %s",
		cfg.table, cfg.afterColumn,
	)).Error; err != nil {
		return err
	}

	// Backfill from parent table
	if err := db.Exec(fmt.Sprintf(
		"UPDATE `%s` t INNER JOIN `%s` p ON t.%s = p.id SET t.team_id = p.team_id",
		cfg.table, cfg.parentTable, cfg.parentFK,
	)).Error; err != nil {
		return err
	}

	// Make NOT NULL
	if err := db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s` MODIFY COLUMN team_id CHAR(26) NOT NULL",
		cfg.table,
	)).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(fmt.Sprintf(
		"CREATE INDEX idx_%s_team_id ON `%s`(team_id)",
		cfg.table, cfg.table,
	)).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s` ADD CONSTRAINT fk_%s_team_id FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE",
		cfg.table, cfg.table,
	)).Error; err != nil {
		return err
	}

	return nil
}

func addTeamIdToAllTablesDown(db *gorm.DB) error {
	// Revert NOT NULL on existing tables
	for _, table := range existingNullableTables {
		db.Exec(fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN team_id CHAR(26) NULL", table))
	}

	// Remove team_id from tables in reverse order
	for i := len(tablesToUpdate) - 1; i >= 0; i-- {
		cfg := tablesToUpdate[i]
		db.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY fk_%s_team_id", cfg.table, cfg.table))
		db.Exec(fmt.Sprintf("DROP INDEX idx_%s_team_id ON `%s`", cfg.table, cfg.table))
		db.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN team_id", cfg.table))
	}

	return nil
}
