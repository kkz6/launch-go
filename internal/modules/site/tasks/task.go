package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// SiteTask extends the base Task interface with site-specific methods
type SiteTask interface {
	taskrunner.Task
	Site() *models.Site
}

// BaseSiteTask embeds taskrunner.BaseTask and adds site reference
type BaseSiteTask struct {
	taskrunner.BaseTask
	site *models.Site
}

func (t *BaseSiteTask) Site() *models.Site {
	return t.site
}
