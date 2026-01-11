package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// DeleteFirewallRule removes a firewall rule from a server
type DeleteFirewallRule struct {
	BaseServerTask
	rule *models.FirewallRule
}

// NewDeleteFirewallRule creates a new DeleteFirewallRule task
func NewDeleteFirewallRule(server *models.Server, rule *models.FirewallRule) *DeleteFirewallRule {
	task := &DeleteFirewallRule{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/firewall/delete-rule",
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
func (t *DeleteFirewallRule) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":   t.server,
		"Username": t.server.GetUsername(),
		"Rule":     t.rule,
		"UfwRule":  t.rule.FormatAsUfwRule(),
	}
}

// Rule returns the firewall rule being deleted
func (t *DeleteFirewallRule) Rule() *models.FirewallRule {
	return t.rule
}

// onFinished handles successful completion
func (t *DeleteFirewallRule) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Delete the rule from database
	// This would be handled by the job/repository layer
}

// onFailed handles task failure
func (t *DeleteFirewallRule) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update rule status to uninstallation failed
	// Fire FirewallRuleDeleteFailed event
}

// onTimeout handles task timeout
func (t *DeleteFirewallRule) onTimeout(ctx context.Context, result *taskrunner.TaskResult) {
	// Update rule status to uninstallation failed
	// Dispatch cleanup job with timeout message
}
