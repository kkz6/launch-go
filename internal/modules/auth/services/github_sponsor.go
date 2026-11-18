package services

import (
	"fmt"
	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
)

// All destination boundaries use the same row-lock order during membership,
// deletion and transfer, so new GitHub grants cannot race a changed workspace.
func lockGitHubTeams(tx *gorm.DB, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	sort.Strings(ids)
	for _, id := range ids {
		var team authmodels.Team
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&team, "id = ?", id).Error; err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}
	}
	return nil
}
