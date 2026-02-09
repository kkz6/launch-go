package adapters

import (
	"context"

	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"gorm.io/gorm"
)

// SiteCheckerAdapter checks if sites reference a source control.
// Satisfies the git module's SiteChecker interface via structural typing.
type SiteCheckerAdapter struct {
	db *gorm.DB
}

// NewSiteCheckerAdapter creates a new SiteCheckerAdapter
func NewSiteCheckerAdapter(db *gorm.DB) *SiteCheckerAdapter {
	return &SiteCheckerAdapter{db: db}
}

// HasSitesBySourceControlID checks if any sites reference the given source control ID
func (a *SiteCheckerAdapter) HasSitesBySourceControlID(ctx context.Context, sourceControlID string) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Model(&sitemodels.Site{}).
		Where("source_control_id = ?", sourceControlID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
