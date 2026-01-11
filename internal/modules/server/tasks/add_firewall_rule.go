package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// AddFirewallRule installs a firewall rule on a server
type AddFirewallRule struct {
	BaseServerTask
	rule *models.FirewallRule
}

// NewAddFirewallRule creates a new AddFirewallRule task
func NewAddFirewallRule(server *models.Server, rule *models.FirewallRule) *AddFirewallRule {
	task := &AddFirewallRule{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/firewall/add-rule",
				TaskTimeout:  2 * time.Minute,
			},
			server: server,
		},
		rule: rule,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed
	task.TimeoutCallback = task.onTimeout

	return task
}

// Data returns the template data
func (t *AddFirewallRule) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":   t.server,
		"Username": t.server.GetUsername(),
		"Rule":     t.rule,
		"UfwRule":  t.rule.FormatAsUfwRule(),
	}
}

// Rule returns the firewall rule being added
func (t *AddFirewallRule) Rule() *models.FirewallRule {
	return t.rule
}

// onFinished handles successful completion
func (t *AddFirewallRule) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update rule status to installed
	// This would be handled by the job/repository layer
}

// onFailed handles task failure
func (t *AddFirewallRule) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update rule status to failed
	// Fire FirewallRuleInstallFailed event
}

// onTimeout handles task timeout
func (t *AddFirewallRule) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update rule status to failed
	// Dispatch cleanup job with timeout message
}
