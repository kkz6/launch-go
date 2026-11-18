package gitaccess

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"gorm.io/gorm"
	"time"
)

// WithWorkloadLock serializes workflow provisioning and local disable/delete.
// Session locking leaves the cleanup manifest durable across remote failures.
func WithWorkloadLock(ctx context.Context, db *gorm.DB, kind, id string, work func(*gorm.DB) error) error {
	if db.Dialector.Name() != "postgres" {
		return errors.New("GitHub Actions provisioning requires PostgreSQL locking")
	}
	sum := sha256.Sum256([]byte("launch-gha-bootstrap/" + kind + "/" + id))
	lockID := int64(binary.BigEndian.Uint64(sum[:8]))
	return db.Connection(func(connection *gorm.DB) error {
		var acquired bool
		if err := connection.WithContext(ctx).Raw("SELECT pg_try_advisory_lock(?)", lockID).Scan(&acquired).Error; err != nil {
			return err
		}
		if !acquired {
			return errors.New("another workflow update is in progress; retry shortly")
		}
		defer func() {
			unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			connection.WithContext(unlockCtx).Exec("SELECT pg_advisory_unlock(?)", lockID)
		}()
		return work(connection)
	})
}
