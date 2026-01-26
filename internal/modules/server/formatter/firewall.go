package formatter

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// UfwRule formats a firewall rule as a UFW command string.
// Example output: "allow from 192.168.1.1 to any port 22"
func UfwRule(rule *models.FirewallRule) string {
	var parts []string

	parts = append(parts, rule.Action.String())

	if rule.FromIPv4 != nil && *rule.FromIPv4 != "" {
		parts = append(parts, fmt.Sprintf("from %s to any port", *rule.FromIPv4))
	}

	if rule.Port != "" {
		parts = append(parts, rule.Port)
	}

	return strings.Join(parts, " ")
}
