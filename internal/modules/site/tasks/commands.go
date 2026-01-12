package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RunCommandForSiteConfig holds configuration for running a command in a site's directory
type RunCommandForSiteConfig struct {
	ApplicationDirectory string
	Command              string
}

// RunCommandForSite creates a task to run a command in a site's application directory
func RunCommandForSite(config RunCommandForSiteConfig) *taskrunner.BaseTask {
	script := templates.MustRender("run_command_for_site.sh", struct {
		ApplicationDirectory string
		Command              string
	}{
		ApplicationDirectory: config.ApplicationDirectory,
		Command:              config.Command,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Run Command"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// RestartAllSiteQueuesConfig holds configuration for restarting all queues
type RestartAllSiteQueuesConfig struct {
	SiteAddress string
	HasQueues   bool
	Commands    []string
}

// RestartAllSiteQueues creates a task to restart all queue workers for a site
func RestartAllSiteQueues(config RestartAllSiteQueuesConfig) *taskrunner.BaseTask {
	script := templates.MustRender("restart_all_site_queues.sh", struct {
		SiteAddress string
		HasQueues   bool
		Commands    []string
	}{
		SiteAddress: config.SiteAddress,
		HasQueues:   config.HasQueues,
		Commands:    config.Commands,
	})

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart All Site Queues"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(120),
	)
}
