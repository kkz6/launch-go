package tasks

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// RestartAllSiteQueues restarts all supervisor queue workers for a site
type RestartAllSiteQueues struct {
	BaseSiteTask
	queues []models.Queue
}

// NewRestartAllSiteQueues creates a new RestartAllSiteQueues task
func NewRestartAllSiteQueues(site *models.Site, queues []models.Queue) *RestartAllSiteQueues {
	return &RestartAllSiteQueues{
		BaseSiteTask: BaseSiteTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "site/restart-all-site-queues",
				TaskTimeout:  60 * time.Second,
			},
			site: site,
		},
		queues: queues,
	}
}

// Data returns the template data
func (t *RestartAllSiteQueues) Data() map[string]interface{} {
	return map[string]interface{}{
		"Site":      t.siteData(),
		"HasQueues": t.hasQueues(),
		"Commands":  t.commands(),
	}
}

func (t *RestartAllSiteQueues) siteData() map[string]interface{} {
	return map[string]interface{}{
		"Address": t.site.Address,
	}
}

// hasQueues returns true if the site has queue workers
func (t *RestartAllSiteQueues) hasQueues() bool {
	return len(t.queues) > 0
}

// commands returns the supervisorctl restart commands for each queue
func (t *RestartAllSiteQueues) commands() []string {
	var commands []string

	for _, queue := range t.queues {
		commands = append(commands, fmt.Sprintf("supervisorctl restart %s:*", queue.ID))
	}

	return commands
}
