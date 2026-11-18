package services

import (
	"context"
	gitaccess "github.com/kkz6/launch-go/internal/modules/git/access"
	"gorm.io/gorm"
)

func revokeDeletedGitHubSponsor(ctx context.Context, tx *gorm.DB, userID string) error {
	return gitaccess.RevokeLocalUser(ctx, tx, userID)
}
