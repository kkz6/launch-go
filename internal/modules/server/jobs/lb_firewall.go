package jobs

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

// Job type constants for load balancer firewall operations
const (
	TypeAddLBFirewallRule    = "server:add_lb_firewall_rule"
	TypeRemoveLBFirewallRule = "server:remove_lb_firewall_rule"
)

// LBFirewallPayload holds the data for LB firewall rule operations
type LBFirewallPayload struct {
	BackendServerID string `json:"backend_server_id"`
	LoadBalancerIP  string `json:"load_balancer_ip"`
	Port            int    `json:"port"`
}

// --- Add LB Firewall Rule ---

// AddLBFirewallRuleJob adds a UFW rule on the backend server to allow traffic from the LB
type AddLBFirewallRuleJob struct {
	Deps    *JobDeps
	Payload LBFirewallPayload
}

func NewAddLBFirewallRuleJob(p LBFirewallPayload) pkgjobs.Handler {
	return &AddLBFirewallRuleJob{Deps: deps, Payload: p}
}

func (j *AddLBFirewallRuleJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.BackendServerID)
	if err != nil {
		return fmt.Errorf("failed to find backend server: %w", err)
	}

	task := tasks.AddFirewallRule(
		tasks.FirewallAllow,
		strconv.Itoa(j.Payload.Port),
		"tcp",
		j.Payload.LoadBalancerIP,
	)

	result, err := j.Deps.RunTask(server, task).AsUser().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to add LB firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to add LB firewall rule: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("backend_server_id", j.Payload.BackendServerID).
		Str("lb_ip", j.Payload.LoadBalancerIP).
		Int("port", j.Payload.Port).
		Msg("LB firewall rule added on backend server")

	return nil
}

func (j *AddLBFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("backend_server_id", j.Payload.BackendServerID).
		Str("lb_ip", j.Payload.LoadBalancerIP).
		Int("port", j.Payload.Port).
		Msg("failed to add LB firewall rule")
}

// NewAddLBFirewallRuleTask creates an asynq task for adding a LB firewall rule
func NewAddLBFirewallRuleTask(backendServerID, loadBalancerIP string, port int) (*asynq.Task, error) {
	return pkgjobs.Task(TypeAddLBFirewallRule, LBFirewallPayload{
		BackendServerID: backendServerID,
		LoadBalancerIP:  loadBalancerIP,
		Port:            port,
	}, asynq.TaskID(pkgjobs.Dedup("add_lb_fw", backendServerID, loadBalancerIP, strconv.Itoa(port))), asynq.MaxRetry(3))
}

// --- Remove LB Firewall Rule ---

// RemoveLBFirewallRuleJob removes the UFW rule on the backend server for LB traffic
type RemoveLBFirewallRuleJob struct {
	Deps    *JobDeps
	Payload LBFirewallPayload
}

func NewRemoveLBFirewallRuleJob(p LBFirewallPayload) pkgjobs.Handler {
	return &RemoveLBFirewallRuleJob{Deps: deps, Payload: p}
}

func (j *RemoveLBFirewallRuleJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.BackendServerID)
	if err != nil {
		return fmt.Errorf("failed to find backend server: %w", err)
	}

	task := tasks.DeleteFirewallRule(
		tasks.FirewallAllow,
		strconv.Itoa(j.Payload.Port),
		"tcp",
		j.Payload.LoadBalancerIP,
	)

	result, err := j.Deps.RunTask(server, task).AsUser().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove LB firewall rule: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to remove LB firewall rule: %s", result.GetOutput())
	}

	j.Deps.Logger.Info().
		Str("backend_server_id", j.Payload.BackendServerID).
		Str("lb_ip", j.Payload.LoadBalancerIP).
		Int("port", j.Payload.Port).
		Msg("LB firewall rule removed from backend server")

	return nil
}

func (j *RemoveLBFirewallRuleJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("backend_server_id", j.Payload.BackendServerID).
		Str("lb_ip", j.Payload.LoadBalancerIP).
		Int("port", j.Payload.Port).
		Msg("failed to remove LB firewall rule")
}

// NewRemoveLBFirewallRuleTask creates an asynq task for removing a LB firewall rule
func NewRemoveLBFirewallRuleTask(backendServerID, loadBalancerIP string, port int) (*asynq.Task, error) {
	return pkgjobs.Task(TypeRemoveLBFirewallRule, LBFirewallPayload{
		BackendServerID: backendServerID,
		LoadBalancerIP:  loadBalancerIP,
		Port:            port,
	}, asynq.TaskID(pkgjobs.Dedup("remove_lb_fw", backendServerID, loadBalancerIP, strconv.Itoa(port))), asynq.MaxRetry(3))
}
