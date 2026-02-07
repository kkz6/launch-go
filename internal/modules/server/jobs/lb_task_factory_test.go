package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

func TestNewInstallLBCaddyfileTask(t *testing.T) {
	serverID := "srv_01JTEST00000000000000001"
	upstreamID := "ups_01JTEST00000000000000001"

	task, err := NewInstallLBCaddyfileTask(serverID, upstreamID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeInstallLBCaddyfile, task.Type())
	assert.Equal(t, "server:install_lb_caddyfile", task.Type())

	var payload InstallLBCaddyfilePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Equal(t, upstreamID, payload.UpstreamID)

	// Verify the dedup ID that would be set on the task
	expectedDedupID := pkgjobs.Dedup("install_lb_caddyfile", serverID, upstreamID)
	assert.NotEmpty(t, expectedDedupID)
	assert.Equal(t, "install_lb_caddyfile:srv_01JTEST00000000000000001:ups_01JTEST00000000000000001", expectedDedupID)
}

func TestNewUpdateLBCaddyfileTask(t *testing.T) {
	serverID := "srv_01JTEST00000000000000002"
	upstreamID := "ups_01JTEST00000000000000002"

	task, err := NewUpdateLBCaddyfileTask(serverID, upstreamID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeUpdateLBCaddyfile, task.Type())
	assert.Equal(t, "server:update_lb_caddyfile", task.Type())

	var payload UpdateLBCaddyfilePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Equal(t, upstreamID, payload.UpstreamID)

	expectedDedupID := pkgjobs.Dedup("update_lb_caddyfile", serverID, upstreamID)
	assert.NotEmpty(t, expectedDedupID)
	assert.Equal(t, "update_lb_caddyfile:srv_01JTEST00000000000000002:ups_01JTEST00000000000000002", expectedDedupID)
}

func TestNewRemoveLBCaddyfileTask(t *testing.T) {
	serverID := "srv_01JTEST00000000000000003"
	upstreamID := "ups_01JTEST00000000000000003"

	task, err := NewRemoveLBCaddyfileTask(serverID, upstreamID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeRemoveLBCaddyfile, task.Type())
	assert.Equal(t, "server:remove_lb_caddyfile", task.Type())

	var payload RemoveLBCaddyfilePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, serverID, payload.ServerID)
	assert.Equal(t, upstreamID, payload.UpstreamID)

	expectedDedupID := pkgjobs.Dedup("remove_lb_caddyfile", serverID, upstreamID)
	assert.NotEmpty(t, expectedDedupID)
	assert.Equal(t, "remove_lb_caddyfile:srv_01JTEST00000000000000003:ups_01JTEST00000000000000003", expectedDedupID)
}

func TestNewAddLBFirewallRuleTask(t *testing.T) {
	backendServerID := "srv_01JTEST00000000000000004"
	loadBalancerIP := "10.0.0.50"
	port := 8080

	task, err := NewAddLBFirewallRuleTask(backendServerID, loadBalancerIP, port)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeAddLBFirewallRule, task.Type())
	assert.Equal(t, "server:add_lb_firewall_rule", task.Type())

	var payload LBFirewallPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, backendServerID, payload.BackendServerID)
	assert.Equal(t, loadBalancerIP, payload.LoadBalancerIP)
	assert.Equal(t, port, payload.Port)

	expectedDedupID := pkgjobs.Dedup("add_lb_fw", backendServerID, loadBalancerIP, "8080")
	assert.NotEmpty(t, expectedDedupID)
	assert.Equal(t, "add_lb_fw:srv_01JTEST00000000000000004:10.0.0.50:8080", expectedDedupID)
}

func TestNewRemoveLBFirewallRuleTask(t *testing.T) {
	backendServerID := "srv_01JTEST00000000000000005"
	loadBalancerIP := "10.0.0.60"
	port := 443

	task, err := NewRemoveLBFirewallRuleTask(backendServerID, loadBalancerIP, port)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeRemoveLBFirewallRule, task.Type())
	assert.Equal(t, "server:remove_lb_firewall_rule", task.Type())

	var payload LBFirewallPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, backendServerID, payload.BackendServerID)
	assert.Equal(t, loadBalancerIP, payload.LoadBalancerIP)
	assert.Equal(t, port, payload.Port)

	expectedDedupID := pkgjobs.Dedup("remove_lb_fw", backendServerID, loadBalancerIP, "443")
	assert.NotEmpty(t, expectedDedupID)
	assert.Equal(t, "remove_lb_fw:srv_01JTEST00000000000000005:10.0.0.60:443", expectedDedupID)
}

func TestNewCheckLBBackendHealthTask(t *testing.T) {
	task, err := NewCheckLBBackendHealthTask()
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeCheckLBBackendHealth, task.Type())
	assert.Equal(t, "server:check_lb_backend_health", task.Type())

	var payload CheckLBBackendHealthPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Empty(t, payload.UpstreamID)
}

func TestNewCheckLBBackendHealthTaskForUpstream(t *testing.T) {
	upstreamID := "ups_01JTEST00000000000000010"

	task, err := NewCheckLBBackendHealthTaskForUpstream(upstreamID)
	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, TypeCheckLBBackendHealth, task.Type())

	var payload CheckLBBackendHealthPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)
	assert.Equal(t, upstreamID, payload.UpstreamID)

	expectedDedupID := pkgjobs.Dedup("check_lb_health", upstreamID)
	assert.Equal(t, "check_lb_health:ups_01JTEST00000000000000010", expectedDedupID)
}
