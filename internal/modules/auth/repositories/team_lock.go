package repositories

import (
	"context"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Membership changes share the destination-team lock used by repository
// selection, so a reviewed sharing boundary cannot change during its commit.
func withLockedTeam(ctx context.Context, db *gorm.DB, teamID string, mutate func(*gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var team models.Team
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&team, "id = ?", teamID).Error; err != nil {
			return err
		}
		return mutate(tx)
	})
}
