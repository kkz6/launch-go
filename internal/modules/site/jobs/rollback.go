package jobs

import (
	"context"
)

// RollbackJob handles deployment rollback
type RollbackJob struct {
	SiteJobBase
	Payload RollbackPayload
}

// Type returns the job type
func (j *RollbackJob) Type() string {
	return TypeRollback
}

// Handle executes the rollback job
func (j *RollbackJob) Handle(ctx context.Context) error {
	// TODO: Implement rollback logic
	// 1. Get site and target release
	// 2. Update symlink to target release
	// 3. Restart services
	// 4. Update deployment status
	j.LogInfo("Rollback job executed (not implemented)",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
		"target_deployment_id", j.Payload.TargetDeploymentID,
	)
	return nil
}

// Failed handles job failure
func (j *RollbackJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Rollback job failed",
		"site_id", j.Payload.SiteID,
		"deployment_id", j.Payload.DeploymentID,
	)
}
