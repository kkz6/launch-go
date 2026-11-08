package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteForDeletion resolves the site's owning server and team across tenants.
func (r *Registry) SiteForDeletion(ctx context.Context, id string) (*sitemodels.Site, error) {
	var site sitemodels.Site
	err := r.db.WithContext(ctx).Model(&sitemodels.Site{}).
		Select("sites.*").
		Joins("JOIN servers ON servers.id = sites.server_id AND servers.team_id = sites.team_id").
		Where("sites.id = ?", id).First(&site).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &site, nil
}
