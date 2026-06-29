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
		Up:        addTeamIDToAllTablesUp,
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
	{"deployments", "site_id", "sites", "site_id"},
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

func addTeamIDToAllTablesUp(db *gorm.DB) error {
	// Add team_id to tables that don't have it
	for _, cfg := range tablesToUpdate {
		if err := addTeamIDColumn(db, cfg); err != nil {
			return fmt.Errorf("failed to add team_id to %s: %w", cfg.table, err)
		}
	}

	// Make existing nullable team_id columns NOT NULL
	for _, table := range existingNullableTables {
		if err := db.Exec(fmt.Sprintf(
			`ALTER TABLE "%s" ALTER COLUMN team_id SET NOT NULL`, table,
		)).Error; err != nil {
			return fmt.Errorf("failed to make team_id NOT NULL on %s: %w", table, err)
		}
	}

	return nil
}

func addTeamIDColumn(db *gorm.DB, cfg tableConfig) error {
	// Add nullable column. Postgres has no; columns append.
	if err := db.Exec(fmt.Sprintf(
		`ALTER TABLE "%s" ADD COLUMN team_id CHAR(26) NULL`,
		cfg.table,
	)).Error; err != nil {
		return err
	}

	// Backfill from parent table. Postgres UPDATE...FROM syntax instead of
	// MySQL's UPDATE...INNER JOIN.
	if err := db.Exec(fmt.Sprintf(
		`UPDATE "%s" t SET team_id = p.team_id FROM "%s" p WHERE t.%s = p.id`,
		cfg.table, cfg.parentTable, cfg.parentFK,
	)).Error; err != nil {
		return err
	}

	// Make NOT NULL — Postgres splits type and nullability.
	if err := db.Exec(fmt.Sprintf(
		`ALTER TABLE "%s" ALTER COLUMN team_id SET NOT NULL`,
		cfg.table,
	)).Error; err != nil {
		return err
	}

	// Add index
	if err := db.Exec(fmt.Sprintf(
		`CREATE INDEX idx_%s_team_id ON "%s"(team_id)`,
		cfg.table, cfg.table,
	)).Error; err != nil {
		return err
	}

	// Add foreign key constraint
	return db.Exec(fmt.Sprintf(
		`ALTER TABLE "%s" ADD CONSTRAINT fk_%s_team_id FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE`,
		cfg.table, cfg.table,
	)).Error
}
