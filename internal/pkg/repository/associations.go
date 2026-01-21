package repository

import (
	"context"

	"gorm.io/gorm"
)

// SyncAssociations replaces all existing associations for a given foreign key with new ones.
// This is useful for many-to-many relationships where you want to replace all linked records.
//
// The function:
// 1. Deletes all existing records matching the foreign key
// 2. Creates all new association records
// 3. Wraps both operations in a transaction for atomicity
//
// Usage:
//
//	backupDatabases := make([]models.BackupDatabase, len(databaseIDs))
//	for i, dbID := range databaseIDs {
//	    backupDatabases[i] = models.BackupDatabase{BackupID: backupID, DatabaseID: dbID}
//	}
//	err := repository.SyncAssociations(ctx, db, "backup_id", backupID, backupDatabases)
func SyncAssociations[T any](ctx context.Context, db *gorm.DB, foreignKey string, foreignID string, associations []T) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing associations
		if err := tx.Where(foreignKey+" = ?", foreignID).Delete(new(T)).Error; err != nil {
			return err
		}

		// Create new associations
		for _, assoc := range associations {
			if err := tx.Create(&assoc).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// SyncAssociationsWithTx is like SyncAssociations but uses an existing transaction.
// Use this when you need to sync associations as part of a larger transaction.
//
// Usage:
//
//	err := db.Transaction(func(tx *gorm.DB) error {
//	    if err := tx.Save(backup).Error; err != nil {
//	        return err
//	    }
//	    return repository.SyncAssociationsWithTx(tx, "backup_id", backup.ID, backupDatabases)
//	})
func SyncAssociationsWithTx[T any](tx *gorm.DB, foreignKey string, foreignID string, associations []T) error {
	// Delete existing associations
	if err := tx.Where(foreignKey+" = ?", foreignID).Delete(new(T)).Error; err != nil {
		return err
	}

	// Create new associations
	for _, assoc := range associations {
		if err := tx.Create(&assoc).Error; err != nil {
			return err
		}
	}

	return nil
}
