package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/backup/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// BackupRepository handles database operations for backups. Generic CRUD
// (Create / FindByID / Update / UpdateFields / Delete / Exists) comes
// from repository.Base[Backup]; only domain-specific queries are defined
// here. Reads preload Jobs (ordered + capped), StorageProvider, and
// Databases. The Jobs preload uses a callback so it isn't expressible
// via Base default-preloads.
type BackupRepository struct {
	repository.Base[models.Backup]
}

// NewBackupRepository creates a new backup repository.
func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{Base: repository.NewBase[models.Backup](db)}
}

// CreateBackupWithDatabases creates a backup and associates it with databases.
func (r *BackupRepository) CreateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(backup).Error; err != nil {
			return err
		}
		associations := make([]models.BackupDatabase, len(databaseIDs))
		for i, dbID := range databaseIDs {
			associations[i] = models.BackupDatabase{BackupID: backup.ID, DatabaseID: dbID}
		}
		return repository.SyncAssociationsWithTx(tx, "backup_id", backup.ID, associations)
	})
}

// FindBackupByID finds a backup by ID with preloads.
func (r *BackupRepository) FindBackupByID(ctx context.Context, id string) (*models.Backup, error) {
	return repository.FindOne[models.Backup](ctx, r.DB,
		repository.WithID(id),
		repository.PreloadWithScope("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}),
		repository.Preload("StorageProvider"),
		repository.Preload("Databases"),
	)
}

// FindBackupByIDAndServer finds a backup by ID and server ID with preloads.
func (r *BackupRepository) FindBackupByIDAndServer(ctx context.Context, id, serverID string) (*models.Backup, error) {
	return repository.FindOne[models.Backup](ctx, r.DB,
		repository.WithID(id),
		repository.WithServerID(serverID),
		repository.PreloadWithScope("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}),
		repository.Preload("StorageProvider"),
		repository.Preload("Databases"),
	)
}

// FindBackupsByServerID finds all backups for a server with preloads.
func (r *BackupRepository) FindBackupsByServerID(ctx context.Context, serverID string) ([]models.Backup, error) {
	return repository.FindAll[models.Backup](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.OrderByCreatedDesc(),
		repository.PreloadWithScope("Jobs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}),
		repository.Preload("StorageProvider"),
		repository.Preload("Databases"),
	)
}

// UpdateBackupWithDatabases updates a backup and its associated databases.
func (r *BackupRepository) UpdateBackupWithDatabases(ctx context.Context, backup *models.Backup, databaseIDs []string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(backup).Error; err != nil {
			return err
		}
		associations := make([]models.BackupDatabase, len(databaseIDs))
		for i, dbID := range databaseIDs {
			associations[i] = models.BackupDatabase{BackupID: backup.ID, DatabaseID: dbID}
		}
		return repository.SyncAssociationsWithTx(tx, "backup_id", backup.ID, associations)
	})
}

// UpdateBackupFields updates specific fields of a backup. Aliases Base.UpdateFields
// so existing callers and tests keep working without churn.
func (r *BackupRepository) UpdateBackupFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.UpdateFields(ctx, id, fields)
}

// DeleteBackup soft-deletes a backup. Aliases Base.Delete for compat.
func (r *BackupRepository) DeleteBackup(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

// GetLatestBackupByServerID gets the most recent backup for a server.
// Returns (nil, nil) when no backup exists (this is by design — callers
// treat absence as a normal state, not an error).
func (r *BackupRepository) GetLatestBackupByServerID(ctx context.Context, serverID string) (*models.Backup, error) {
	return repository.FindOneOrNil[models.Backup](ctx, r.DB,
		repository.WithServerID(serverID),
		repository.OrderByCreatedDesc(),
	)
}

// SyncBackupDatabases syncs the databases for a backup.
func (r *BackupRepository) SyncBackupDatabases(ctx context.Context, backupID string, databaseIDs []string) error {
	associations := make([]models.BackupDatabase, len(databaseIDs))
	for i, dbID := range databaseIDs {
		associations[i] = models.BackupDatabase{BackupID: backupID, DatabaseID: dbID}
	}
	return repository.SyncAssociations(ctx, r.DB, "backup_id", backupID, associations)
}

// GetBackupDatabaseIDs gets the database IDs associated with a backup.
func (r *BackupRepository) GetBackupDatabaseIDs(ctx context.Context, backupID string) ([]string, error) {
	var backupDBs []models.BackupDatabase
	err := r.DB.WithContext(ctx).
		Where("backup_id = ?", backupID).
		Find(&backupDBs).Error
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(backupDBs))
	for i, bd := range backupDBs {
		ids[i] = bd.DatabaseID
	}
	return ids, nil
}
