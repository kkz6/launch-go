package repositories

import (
	"context"

	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Membership and its sponsored grants change in the same team-locked
// transaction. A reconnect cannot commit between removal and invalidation.
func suspendGitHubSponsor(ctx context.Context, tx *gorm.DB, teamID, userID string) error {
	if !tx.Migrator().HasTable("git_hub_authorizations") {
		return nil
	}
	authorizations := tx.Table("git_hub_authorizations").Select("id").Where("user_id = ?", userID)
	var ids []string
	if err := tx.Table("source_controls").Clauses(clause.Locking{Strength: "UPDATE"}).Where("team_id = ? AND git_hub_authorization_id IN (?) AND status = ?", teamID, authorizations, "active").Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Table("source_controls").Where("id IN ?", ids).Updates(map[string]any{"status": "reconnect_required", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := gitaccess.ActivateCleanup(ctx, tx, id, ""); err != nil {
			return err
		}
	}
	return tx.Table("git_hub_credential_leases").Where("source_control_id IN ? AND status IN ?", ids, []string{"issuing", "released"}).Update("status", "revoke_pending").Error
}
