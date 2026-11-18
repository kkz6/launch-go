package services

import (
	"context"
	"github.com/kkz6/launch-go/internal/modules/git/access"
	"github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/git/providers"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

// Inventory reconciliation can invalidate an existing grant, but cannot create
// or restore one. Deleted repository records remain for workload history.
func (s *SourceControlService) syncGitHubInventory(ctx context.Context, sc *models.SourceControl, repositories []map[string]any) error {
	return s.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var connection models.SourceControl
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&connection, "id = ?", sc.ID).Error; err != nil {
			return err
		}
		if connection.Generation != sc.Generation || connection.SelectionVersion != sc.SelectionVersion {
			return access.ErrStaleAccess
		}
		if connection.GitHubInstallationID == nil {
			return nil
		}
		var installation models.GitHubInstallation
		if err := tx.First(&installation, "id = ?", *connection.GitHubInstallationID).Error; err != nil {
			return err
		}
		if installation.Status != "active" {
			return nil
		}
		now := time.Now()
		seen := map[string]map[string]any{}
		for _, data := range repositories {
			remoteID := providers.ExtractFloatID(data, "id")
			if remoteID == "" {
				continue
			}
			seen[remoteID] = data
			var catalog models.GitHubInstallationRepository
			err := tx.First(&catalog, "installation_id = ? AND repository_id = ?", installation.ID, remoteID).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			catalog.InstallationID = installation.ID
			catalog.RepositoryID = remoteID
			catalog.FullName = str(data, "full_name")
			catalog.DefaultBranch = str(data, "default_branch")
			catalog.Private = boolean(data, "private")
			catalog.Available = true
			if err := tx.Save(&catalog).Error; err != nil {
				return err
			}
		}
		ids := make([]string, 0, len(seen))
		for id := range seen {
			ids = append(ids, id)
		}
		q := tx.Model(&models.GitHubInstallationRepository{}).Where("installation_id = ?", installation.ID)
		if len(ids) > 0 {
			q = q.Where("repository_id NOT IN ?", ids)
		}
		if err := q.Update("available", false).Error; err != nil {
			return err
		}
		var grants []models.SourceControlRepository
		if err := tx.Where("source_control_id = ?", sc.ID).Find(&grants).Error; err != nil {
			return err
		}
		invalidated := []uint64{}
		for _, grant := range grants {
			data, exists := seen[grant.GitHubRepositoryID]
			if !exists || str(data, "full_name") != grant.FullName {
				if grant.Available {
					invalidated = append(invalidated, grant.ID)
					if err := tx.Model(&models.SourceControlRepository{}).Where("id = ?", grant.ID).Updates(map[string]any{"available": false, "verified_at": nil, "grant_version": gorm.Expr("grant_version + 1")}).Error; err != nil {
						return err
					}
					if err := access.ActivateCleanup(ctx, tx, sc.ID, strconv.FormatUint(grant.ID, 10)); err != nil {
						return err
					}
				}
				continue
			}
			// Metadata refresh never restores availability after an upstream removal.
			if err := tx.Model(&models.SourceControlRepository{}).Where("id = ?", grant.ID).Updates(map[string]any{"default_branch": str(data, "default_branch"), "public": !boolean(data, "private")}).Error; err != nil {
				return err
			}
		}
		if len(invalidated) > 0 {
			if err := tx.Model(&models.GitHubCredentialLease{}).Where("source_control_id = ? AND repository_id IN ? AND status IN ?", sc.ID, invalidated, []string{"issuing", "released"}).Update("status", "revoke_pending").Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&models.SourceControlRepository{}).Where("source_control_id = ? AND selected = ? AND available = ?", sc.ID, true, true).Count(&count).Error; err != nil {
			return err
		}
		return tx.Model(&connection).Updates(map[string]any{"repository_count": count, "last_synced_at": now}).Error
	})
}
