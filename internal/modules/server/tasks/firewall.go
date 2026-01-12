package tasks

import (
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// FirewallAction represents the action for a firewall rule
type FirewallAction string

const (
	FirewallAllow FirewallAction = "allow"
	FirewallDeny  FirewallAction = "deny"
)

// FirewallRuleConfig holds configuration for a firewall rule
type FirewallRuleConfig struct {
	Action   FirewallAction
	Port     string
	Protocol string // tcp, udp, or empty for both
	FromIP   string // optional source IP
}

// AddFirewallRuleTask adds a firewall rule
type AddFirewallRuleTask struct {
	*ServerTask
	Rule FirewallRuleConfig
}

// NewAddFirewallRuleTask creates a new AddFirewallRuleTask
func NewAddFirewallRuleTask(rule FirewallRuleConfig) *AddFirewallRuleTask {
	script := formatUfwRule(rule, false)

	return &AddFirewallRuleTask{
		ServerTask: NewServerTaskWithName("Add Firewall Rule", script, 30),
		Rule:       rule,
	}
}

// AddFirewallRule creates a task to add a firewall rule
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
		taskrunner.WithTimeout(30),
	)
}

// DeleteFirewallRuleTask deletes a firewall rule
type DeleteFirewallRuleTask struct {
	*ServerTask
	Rule FirewallRuleConfig
}

// NewDeleteFirewallRuleTask creates a new DeleteFirewallRuleTask
func NewDeleteFirewallRuleTask(rule FirewallRuleConfig) *DeleteFirewallRuleTask {
	script := formatUfwRule(rule, true)

	return &DeleteFirewallRuleTask{
		ServerTask: NewServerTaskWithName("Delete Firewall Rule", script, 30),
		Rule:       rule,
	}
}

// DeleteFirewallRule creates a task to delete a firewall rule
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
		taskrunner.WithTimeout(30),
	)
}

// formatUfwRule formats a UFW rule command
func formatUfwRule(rule FirewallRuleConfig, delete bool) string {
	var parts []string

	parts = append(parts, "sudo ufw")

	if delete {
		parts = append(parts, "delete")
	} else if rule.Action == FirewallDeny {
		// Insert deny rules at position 1 (before allow rules)
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
