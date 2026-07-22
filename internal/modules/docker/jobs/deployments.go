package jobs

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
)

// createDeploymentRow inserts a fresh `pending` row into the polymorphic
// docker_deployments table — same table apps + composes use, with
// target_type set to "database" / "application" / "compose". The row
// is created BEFORE the SSH task runs so the frontend's Deployments
// table picks it up immediately (status=pending → running → terminal).
//
// taskID is wired in later by finalizeDeploymentRow once the worker
// has the taskrunner.Task ID — frontend uses it with ServerLogViewer
// entity="task" to stream the SSH output.
func createDeploymentRow(
	ctx context.Context, deps *JobDeps,
	targetType, targetID, action, teamID, serverID string,
) (*models.Deployment, error) {
	now := time.Now().UTC()
	row := &models.Deployment{
		TargetType: targetType,
		TargetID:   targetID,
		Status:     dockertypes.DeploymentStatusPending,
		StartedAt:  &now,
	}
	row.TeamID = teamID
	row.ServerID = serverID
	if action != "" {
		a := action
		row.Action = &a
	}
	if err := deps.Repos.Deployment().Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// attachDeploymentTaskID is called once the worker has dispatched the
// SSH task and learned its task ID — it persists task_id + flips status
// to "running". Frontend listens for the broadcast and shows "View
// logs" the moment this update lands.
func attachDeploymentTaskID(
	ctx context.Context, deps *JobDeps, deploymentID, taskID string,
) {
	if deploymentID == "" {
		return
	}
	updates := map[string]any{
		"status": dockertypes.DeploymentStatusDeploying,
	}
	if taskID != "" {
		updates["task_id"] = taskID
	}
	if err := deps.Repos.Deployment().UpdateFields(ctx, deploymentID, updates); err != nil {
		deps.Logger.Error().Err(err).
			Str("deployment_id", deploymentID).
			Str("task_id", taskID).
			Msg("failed to attach task to deployment")
	}
}

// broadcastDeploymentEvent emits a docker.{kind}.deployment.* event on
// the team channel with the routing fields the frontend's
// useChannelEvents filter expects. Single helper across all three
// workload kinds (apps / composes / databases) — the event name is
// what tells the listener which kind it is.
func broadcastDeploymentEvent(
	deps *JobDeps, event string, deployment *models.Deployment,
	projectID string, errMsg string,
) {
	if deployment == nil {
		return
	}
	payload := map[string]any{
		"deployment_id": deployment.ID,
		"target_type":   deployment.TargetType,
		"target_id":     deployment.TargetID,
		"team_id":       deployment.TeamID,
		"server_id":     deployment.ServerID,
		"project_id":    projectID,
		"status":        string(deployment.Status),
	}
	if deployment.Action != nil {
		payload["action"] = *deployment.Action
	}
	if deployment.TaskID != nil {
		payload["task_id"] = *deployment.TaskID
	}
	// Mirror the deployment's target id under the kind-specific field
	// the frontend's filter checks (e.g. database_id, application_id).
	switch deployment.TargetType {
	case "database":
		payload["database_id"] = deployment.TargetID
	case "application":
		payload["application_id"] = deployment.TargetID
	case "compose":
		payload["compose_id"] = deployment.TargetID
	}
	if errMsg != "" {
		// Single-line summary on the WS payload — the row itself
		// holds the full text.
		if len(errMsg) > 240 {
			errMsg = errMsg[:240] + "…"
		}
		payload["error"] = errMsg
	}
	deps.BroadcastToTeam(deployment.TeamID, event, payload)
}

// finalizeDeploymentRow records the terminal outcome of an SSH task
// run. Always updates finished_at + status. Error is truncated to
// 4000 chars so a runaway script log doesn't bloat the row.
func finalizeDeploymentRow(
	ctx context.Context, deps *JobDeps,
	deploymentID string,
	status dockertypes.DeploymentStatus,
	errMsg string,
) {
	if deploymentID == "" {
		return
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":      status,
		"finished_at": now,
	}
	if errMsg != "" {
		if len(errMsg) > 4000 {
			errMsg = errMsg[:4000] + "…"
		}
		updates["error"] = errMsg
	}
	if err := deps.Repos.Deployment().UpdateFields(ctx, deploymentID, updates); err != nil {
		deps.Logger.Error().Err(err).
			Str("deployment_id", deploymentID).
			Str("status", string(status)).
			Msg("failed to finalize deployment")
	}
}
