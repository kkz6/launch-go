package jobs

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

var errTLSReservationLost = errors.New("TLS update reservation was lost")

func completeTLSReservation(ctx context.Context, db *gorm.DB, siteID string) error {
	return finalizeTLSReservation(ctx, db, siteID, false)
}

func rollbackTLSReservation(ctx context.Context, db *gorm.DB, siteID string) error {
	return finalizeTLSReservation(ctx, db, siteID, true)
}

func finalizeTLSReservation(
	ctx context.Context,
	db *gorm.DB,
	siteID string,
	rollback bool,
) error {
	if db == nil {
		return errors.New("database is not configured")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var site models.Site
		if err := tx.Select(
			"id",
			"pending_tls_update_since",
			"pending_tls_previous_setting",
			"pending_tls_previous_certificate_ids",
			"pending_tls_replacement_certificate_id",
		).First(&site, "id = ?", siteID).Error; err != nil {
			return err
		}
		if site.PendingTLSUpdateSince == nil {
			return errTLSReservationLost
		}
		if rollback && site.PendingTLSPreviousSetting == nil {
			return errors.New("TLS rollback state is unavailable")
		}

		updates := map[string]any{
			"pending_caddyfile_update_since":         nil,
			"pending_tls_update_since":               nil,
			"pending_tls_previous_setting":           nil,
			"pending_tls_previous_certificate_ids":   nil,
			"pending_tls_replacement_certificate_id": nil,
		}
		if rollback {
			updates["tls_setting"] = *site.PendingTLSPreviousSetting
		}

		reservation := tx.Model(&models.Site{}).
			Where(
				"id = ? AND pending_tls_update_since = ?",
				siteID,
				*site.PendingTLSUpdateSince,
			).
			Updates(updates)
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return errTLSReservationLost
		}
		if !rollback {
			return nil
		}

		if site.PendingTLSReplacementCertID != nil {
			if err := tx.Delete(
				&models.Certificate{},
				"id = ? AND site_id = ?",
				*site.PendingTLSReplacementCertID,
				siteID,
			).Error; err != nil {
				return fmt.Errorf("remove replacement certificate: %w", err)
			}
		}
		if err := tx.Model(&models.Certificate{}).
			Where("site_id = ?", siteID).
			Update("is_active", false).Error; err != nil {
			return fmt.Errorf("deactivate site certificates: %w", err)
		}
		if len(site.PendingTLSPreviousCertIDs) == 0 {
			return nil
		}
		if err := tx.Model(&models.Certificate{}).
			Where(
				"site_id = ? AND id IN ?",
				siteID,
				[]string(site.PendingTLSPreviousCertIDs),
			).
			Update("is_active", true).Error; err != nil {
			return fmt.Errorf("restore site certificates: %w", err)
		}
		return nil
	})
}
