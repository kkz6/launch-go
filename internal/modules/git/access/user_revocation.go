package access

import (
	"context"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Local account deletion revokes only that launchctl identity's sponsorship.
// Other launchctl users of the same upstream GitHub identity remain independent.
func RevokeLocalUser(ctx context.Context, tx *gorm.DB, userID string) error {
	if !tx.Migrator().HasTable(&gitmodels.GitHubAuthorization{}) {
		return nil
	}
	var ids []string
	if err := tx.Model(&gitmodels.GitHubAuthorization{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Model(&gitmodels.GitHubAuthorization{}).Where("id IN ?", ids).Updates(map[string]any{"revoked_at": time.Now(), "generation": gorm.Expr("generation + 1"), "access_token": nil, "refresh_token": nil}).Error; err != nil {
		return err
	}
	var connections []gitmodels.SourceControl
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("git_hub_authorization_id IN ?", ids).Find(&connections).Error; err != nil {
		return err
	}
	for _, sc := range connections {
		if err := tx.Model(&gitmodels.SourceControl{}).Where("id = ?", sc.ID).Updates(map[string]any{"status": "reconnect_required", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
			return err
		}
		if err := ActivateCleanup(ctx, tx, sc.ID, ""); err != nil {
			return err
		}
	}
	return tx.Model(&gitmodels.GitHubCredentialLease{}).Where("authorization_id IN ? AND status IN ?", ids, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
}
