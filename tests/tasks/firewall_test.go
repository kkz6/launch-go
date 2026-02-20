package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestAddFirewallRule_AllowTCP(t *testing.T) {
	task := tasks.AddFirewallRule(tasks.FirewallAllow, "443", "tcp", "")

	testutil.AssertTask(t, task).
		HasName("Add Firewall Rule").
		ScriptContains("sudo ufw").
		ScriptContains("allow").
		ScriptContains("proto tcp").
		ScriptContains("to any port 443").
		ScriptMatches("tasks/firewall_allow_tcp")
}

func TestAddFirewallRule_AllowUDP(t *testing.T) {
	task := tasks.AddFirewallRule(tasks.FirewallAllow, "53", "udp", "")

	testutil.AssertTask(t, task).
		HasName("Add Firewall Rule").
		ScriptContains("proto udp").
		ScriptContains("to any port 53").
		ScriptMatches("tasks/firewall_allow_udp")
}

func TestAddFirewallRule_DenyWithIP(t *testing.T) {
	task := tasks.AddFirewallRule(tasks.FirewallDeny, "22", "tcp", "192.168.1.100")

	testutil.AssertTask(t, task).
		HasName("Add Firewall Rule").
		ScriptContains("insert 1"). // Deny rules are inserted first
		ScriptContains("deny").
		ScriptContains("from 192.168.1.100").
		ScriptContains("to any port 22").
		ScriptMatches("tasks/firewall_deny_ip")
}

func TestAddFirewallRule_AllowFromIP(t *testing.T) {
	task := tasks.AddFirewallRule(tasks.FirewallAllow, "3306", "tcp", "10.0.0.0/8")

	testutil.AssertTask(t, task).
		HasName("Add Firewall Rule").
		ScriptContains("allow").
		ScriptContains("from 10.0.0.0/8").
		ScriptContains("to any port 3306").
		ScriptMatches("tasks/firewall_allow_from_ip")
}

func TestAddFirewallRule_RejectWithIP(t *testing.T) {
	task := tasks.AddFirewallRule(tasks.FirewallReject, "22", "tcp", "192.168.1.100")

	testutil.AssertTask(t, task).
		HasName("Add Firewall Rule").
		ScriptContains("insert 1"). // Reject rules are inserted first, like deny
		ScriptContains("reject").
		ScriptContains("from 192.168.1.100").
		ScriptContains("to any port 22").
		ScriptMatches("tasks/firewall_reject_ip")
}

func TestDeleteFirewallRule_Reject(t *testing.T) {
	task := tasks.DeleteFirewallRule(tasks.FirewallReject, "22", "tcp", "192.168.1.100")

	testutil.AssertTask(t, task).
		HasName("Delete Firewall Rule").
		ScriptContains("delete").
		ScriptContains("reject").
		ScriptContains("from 192.168.1.100").
		ScriptMatches("tasks/firewall_delete_reject")
}

func TestDeleteFirewallRule_TCP(t *testing.T) {
	task := tasks.DeleteFirewallRule(tasks.FirewallAllow, "443", "tcp", "")

	testutil.AssertTask(t, task).
		HasName("Delete Firewall Rule").
		ScriptContains("sudo ufw delete").
		ScriptContains("allow").
		ScriptContains("proto tcp").
		ScriptContains("to any port 443").
		ScriptMatches("tasks/firewall_delete_tcp")
}

func TestDeleteFirewallRule_WithIP(t *testing.T) {
	task := tasks.DeleteFirewallRule(tasks.FirewallDeny, "22", "tcp", "192.168.1.100")

	testutil.AssertTask(t, task).
		HasName("Delete Firewall Rule").
		ScriptContains("delete").
		ScriptContains("deny").
		ScriptContains("from 192.168.1.100").
		ScriptMatches("tasks/firewall_delete_with_ip")
}
