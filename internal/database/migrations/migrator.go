package migrations

import (
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Migration represents a single database migration
type Migration struct {
	ID        string
	Name      string
	Up        func(db *gorm.DB) error
	Down      func(db *gorm.DB) error
	Timestamp time.Time
}

// MigrationRecord represents a record in the migrations table
type MigrationRecord struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Migration string    `gorm:"size:255;uniqueIndex;not null"`
	Batch     int       `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (MigrationRecord) TableName() string {
	return "migrations"
}

// Migrator handles database migrations
type Migrator struct {
	db         *gorm.DB
	logger     *zerolog.Logger
	migrations []Migration
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB, logger *zerolog.Logger) *Migrator {
	return &Migrator{
		db:         db,
		logger:     logger,
		migrations: registeredMigrations,
	}
}

// CreateMigrationsTable creates the migrations tracking table
func (m *Migrator) CreateMigrationsTable() error {
	return m.db.AutoMigrate(&MigrationRecord{})
}

// GetRanMigrations returns the list of migrations that have been run
func (m *Migrator) GetRanMigrations() ([]string, error) {
	var records []MigrationRecord
	if err := m.db.Order("migration").Find(&records).Error; err != nil {
		return nil, err
	}

	migrations := make([]string, len(records))
	for i, r := range records {
		migrations[i] = r.Migration
	}

	return migrations, nil
}

// GetPendingMigrations returns migrations that haven't been run yet
func (m *Migrator) GetPendingMigrations() ([]Migration, error) {
	ran, err := m.GetRanMigrations()
	if err != nil {
		return nil, err
	}

	ranMap := make(map[string]bool)
	for _, name := range ran {
		ranMap[name] = true
	}

	var pending []Migration
	for _, migration := range m.migrations {
		if !ranMap[migration.ID] {
			pending = append(pending, migration)
		}
	}

	// Sort by timestamp
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Timestamp.Before(pending[j].Timestamp)
	})

	return pending, nil
}

// GetNextBatch returns the next batch number
func (m *Migrator) GetNextBatch() (int, error) {
	var maxBatch int
	err := m.db.Model(&MigrationRecord{}).Select("COALESCE(MAX(batch), 0)").Scan(&maxBatch).Error
	if err != nil {
		return 0, err
	}

	return maxBatch + 1, nil
}

// Migrate runs all pending migrations
func (m *Migrator) Migrate() error {
	if err := m.CreateMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	pending, err := m.GetPendingMigrations()
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pending) == 0 {
		m.logger.Info().Msg("Nothing to migrate")
		return nil
	}

	batch, err := m.GetNextBatch()
	if err != nil {
		return fmt.Errorf("failed to get next batch: %w", err)
	}

	for _, migration := range pending {
		m.logger.Info().Str("migration", migration.ID).Msg("Migrating")

		if err := migration.Up(m.db); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.ID, err)
		}

		record := MigrationRecord{
			Migration: migration.ID,
			Batch:     batch,
		}
		if err := m.db.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration.ID, err)
		}

		m.logger.Info().Str("migration", migration.ID).Msg("Migrated")
	}

	return nil
}

// Rollback rolls back the last batch of migrations
func (m *Migrator) Rollback() error {
	var lastBatch int
	err := m.db.Model(&MigrationRecord{}).Select("COALESCE(MAX(batch), 0)").Scan(&lastBatch).Error
	if err != nil {
		return fmt.Errorf("failed to get last batch: %w", err)
	}

	if lastBatch == 0 {
		m.logger.Info().Msg("Nothing to rollback")
		return nil
	}

	var records []MigrationRecord
	if err := m.db.Where("batch = ?", lastBatch).Order("id DESC").Find(&records).Error; err != nil {
		return fmt.Errorf("failed to get migrations to rollback: %w", err)
	}

	// Build migration map for quick lookup
	migrationMap := make(map[string]Migration)
	for _, mig := range m.migrations {
		migrationMap[mig.ID] = mig
	}

	for _, record := range records {
		migration, ok := migrationMap[record.Migration]
		if !ok {
			return fmt.Errorf("migration %s not found in registry", record.Migration)
		}

		if migration.Down == nil {
			return fmt.Errorf("migration %s has no down function and cannot be rolled back", migration.ID)
		}

		m.logger.Info().Str("migration", migration.ID).Msg("Rolling back")

		if err := migration.Down(m.db); err != nil {
			return fmt.Errorf("rollback %s failed: %w", migration.ID, err)
		}

		if err := m.db.Delete(&record).Error; err != nil {
			return fmt.Errorf("failed to delete migration record %s: %w", migration.ID, err)
		}

		m.logger.Info().Str("migration", migration.ID).Msg("Rolled back")
	}

	return nil
}

// Fresh drops all tables and re-runs all migrations
func (m *Migrator) Fresh() error {
	m.logger.Info().Msg("Dropping all tables...")

	migrator := m.db.Migrator()

	// Get all tables using GORM's database-agnostic method
	tables, err := migrator.GetTables()
	if err != nil {
		return fmt.Errorf("failed to get tables: %w", err)
	}

	// Drop all tables using GORM's migrator (handles foreign keys internally)
	for _, table := range tables {
		if err := migrator.DropTable(table); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	m.logger.Info().Msg("Running fresh migrations...")
	return m.Migrate()
}

// Status shows the status of all migrations
func (m *Migrator) Status() ([]MigrationStatus, error) {
	if err := m.CreateMigrationsTable(); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	ran, err := m.GetRanMigrations()
	if err != nil {
		return nil, err
	}

	ranMap := make(map[string]bool)
	for _, name := range ran {
		ranMap[name] = true
	}

	var statuses []MigrationStatus
	for _, migration := range m.migrations {
		statuses = append(statuses, MigrationStatus{
			ID:   migration.ID,
			Name: migration.Name,
			Ran:  ranMap[migration.ID],
		})
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	ID   string
	Name string
	Ran  bool
}

// Sync marks all migrations as ran without actually running them.
// This is useful when the database tables already exist (e.g., from Laravel)
// but the Go migrations table doesn't reflect that state.
func (m *Migrator) Sync() error {
	if err := m.CreateMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	pending, err := m.GetPendingMigrations()
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pending) == 0 {
		m.logger.Info().Msg("All migrations are already synced")
		return nil
	}

	batch, err := m.GetNextBatch()
	if err != nil {
		return fmt.Errorf("failed to get next batch: %w", err)
	}

	for _, migration := range pending {
		record := MigrationRecord{
			Migration: migration.ID,
			Batch:     batch,
		}
		if err := m.db.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration.ID, err)
		}

		m.logger.Info().Str("migration", migration.ID).Msg("Marked as ran")
	}

	m.logger.Info().Int("count", len(pending)).Msg("Migrations synced")
	return nil
}

// SyncUntil marks all migrations up to and including the specified migration ID as ran.
func (m *Migrator) SyncUntil(untilID string) error {
	if err := m.CreateMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	pending, err := m.GetPendingMigrations()
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	batch, err := m.GetNextBatch()
	if err != nil {
		return fmt.Errorf("failed to get next batch: %w", err)
	}

	count := 0
	for _, migration := range pending {
		record := MigrationRecord{
			Migration: migration.ID,
			Batch:     batch,
		}
		if err := m.db.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration.ID, err)
		}

		m.logger.Info().Str("migration", migration.ID).Msg("Marked as ran")
		count++

		if migration.ID == untilID {
			break
		}
	}

	m.logger.Info().Int("count", count).Msg("Migrations synced")
	return nil
}
