package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RunCommandForSite runs an arbitrary command in the site's application directory
type RunCommandForSite struct {
	BaseSiteTask
	command string
}

// NewRunCommandForSite creates a new RunCommandForSite task
func NewRunCommandForSite(site *models.Site, command string) *RunCommandForSite {
	return &RunCommandForSite{
		BaseSiteTask: BaseSiteTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "site/run-command-for-site",
				TaskTimeout:  10 * time.Minute,
			},
			site: site,
		},
		command: command,
	}
}

// Data returns the template data
func (t *RunCommandForSite) Data() map[string]interface{} {
	return map[string]interface{}{
		"Site":                 t.siteData(),
		"Command":              t.command,
		"ApplicationDirectory": t.site.GetApplicationDirectory(),
	}
}

func (t *RunCommandForSite) siteData() map[string]interface{} {
	return map[string]interface{}{
		"Path":    t.site.Path,
		"Address": t.site.Address,
	}
}
