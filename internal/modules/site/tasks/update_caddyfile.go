package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UpdateCaddyfile updates the Caddyfile for a site with validation
type UpdateCaddyfile struct {
	BaseSiteTask
	caddyfile string
}

// NewUpdateCaddyfile creates a new UpdateCaddyfile task
func NewUpdateCaddyfile(site *models.Site, caddyfile string) *UpdateCaddyfile {
	return &UpdateCaddyfile{
		BaseSiteTask: BaseSiteTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "site/update-caddyfile",
				TaskTimeout:  30 * time.Second,
			},
			site: site,
		},
		caddyfile: caddyfile,
	}
}

// Data returns the template data
func (t *UpdateCaddyfile) Data() map[string]interface{} {
	return map[string]interface{}{
		"Site":          t.siteData(),
		"Caddyfile":     t.caddyfile,
		"CaddyfilePath": t.caddyfilePath(),
	}
}

func (t *UpdateCaddyfile) siteData() map[string]interface{} {
	return map[string]interface{}{
		"Path":    t.site.Path,
		"Address": t.site.Address,
	}
}

func (t *UpdateCaddyfile) caddyfilePath() string {
	return fmt.Sprintf("%s/Caddyfile", t.site.Path)
}
