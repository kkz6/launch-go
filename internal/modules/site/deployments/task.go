// Package deployments shares site deployment queueing between services, jobs,
// and SSH completion callbacks without coupling those layers to each other.
package deployments

import (
	"github.com/hibiken/asynq"

	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const (
	TypeDeploy             = "site:deploy"
	TypeDeployZeroDowntime = "site:deploy_zero_downtime"
)

type Payload struct {
	SiteID       string            `json:"site_id"`
	DeploymentID string            `json:"deployment_id"`
	UserID       *string           `json:"user_id,omitempty"`
	EnvVariables map[string]string `json:"env_variables,omitempty"`
	ZeroDowntime bool              `json:"zero_downtime,omitempty"`
}

// NewTask preserves the deployment-scoped task ID and transient environment
// variables for both deployment modes.
func NewTask(siteID, deploymentID, userID string, envVars map[string]string, zeroDowntime bool) (*asynq.Task, error) {
	var caller *string
	if userID != "" {
		caller = &userID
	}
	taskType, prefix := TypeDeploy, "deploy"
	if zeroDowntime {
		taskType, prefix = TypeDeployZeroDowntime, "deploy_zd"
	}
	return pkgjobs.TaskWithID(taskType, Payload{
		SiteID:       siteID,
		DeploymentID: deploymentID,
		UserID:       caller,
		EnvVariables: envVars,
		ZeroDowntime: zeroDowntime,
	}, pkgjobs.Dedup(prefix, deploymentID))
}
