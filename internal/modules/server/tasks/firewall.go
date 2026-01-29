package tasks

import (
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for firewall operations
const (
	AddFirewallRuleTaskType    = "server:add_firewall_rule"
	DeleteFirewallRuleTaskType = "server:delete_firewall_rule"
)

// FirewallAction represents the action for a firewall rule.
type FirewallAction string

const (
	FirewallAllow FirewallAction = "allow"
	FirewallDeny  FirewallAction = "deny"
)

// FirewallRuleConfig holds configuration for a firewall rule.
type FirewallRuleConfig struct {
	Action   FirewallAction
	Port     string
	Protocol string
	FromIP   string
}

// AddFirewallRule creates a task to add a UFW firewall rule.
func AddFirewallRule(action FirewallAction, port string, protocol string, fromIP string) *taskrunner.BaseTask {
	rule := FirewallRuleConfig{
		Action:   action,
		Port:     port,
		Protocol: protocol,
		FromIP:   fromIP,
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Add Firewall Rule"),
		taskrunner.WithScript(formatUfwRule(rule, false)),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// DeleteFirewallRule creates a task to delete a UFW firewall rule.
func DeleteFirewallRule(action FirewallAction, port string, protocol string, fromIP string) *taskrunner.BaseTask {
	rule := FirewallRuleConfig{
		Action:   action,
		Port:     port,
		Protocol: protocol,
		FromIP:   fromIP,
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Delete Firewall Rule"),
		taskrunner.WithScript(formatUfwRule(rule, true)),
		taskrunner.WithTimeoutSeconds(30),
	)
}

func formatUfwRule(rule FirewallRuleConfig, remove bool) string {
	var parts []string

	parts = append(parts, "sudo ufw")

	if remove {
		parts = append(parts, "delete")
	} else if rule.Action == FirewallDeny {
		parts = append(parts, "insert 1")
	}

	parts = append(parts, string(rule.Action))

	if rule.Protocol != "" {
		parts = append(parts, "proto", rule.Protocol)
	}

	if rule.FromIP != "" {
		parts = append(parts, "from", rule.FromIP)
	}

	parts = append(parts, "to any port", rule.Port)

	return strings.Join(parts, " ")
}
