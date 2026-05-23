package models

import (
	"time"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Deployment is a single deploy/lifecycle attempt for any docker
// workload — application, compose stack, or managed database.
// Polymorphic via (target_type, target_id) so the deploy history table
// is shared across workload kinds. Read paths always join on
// (target_type, target_id) — never on target_id alone — so cross-kind
// collisions are impossible.
//
// One table, three TargetType values:
//
//   - "application" — git/image/Dockerfile deploys. Action stays empty
//     (implicit "deploy") for backwards compatibility with pre-2j rows.
//   - "compose"     — docker compose up. Same shape as application.
//   - "database"    — managed-database lifecycle. Action is set to
//     create / start / restart / stop / rm. CommitSHA / CommitMsg /
//     ImageRef stay nil because there's nothing to build.
//
// task_id ties the row to the taskrunner.Task (server-tasks table) the
// worker dispatched. Frontend uses this to stream live SSH output via
// the existing /servers/:id/tasks/:taskId/logs websocket — the same
// pattern site deployments use.
type Deployment struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped

	// TargetType is "application", "compose", or "database".
	TargetType string `gorm:"column:target_type;type:varchar(32);not null;index:idx_docker_deployments_target" json:"target_type"`
	TargetID   string `gorm:"column:target_id;type:char(26);not null;index:idx_docker_deployments_target" json:"target_id"`

	// Action records the lifecycle verb when TargetType=="database"
	// (create / start / restart / stop / rm). For app + compose rows
	// the field is nil — every action is implicitly "deploy".
	Action *string `gorm:"type:varchar(32);index" json:"action,omitempty"`

	Status dockertypes.DeploymentStatus `gorm:"type:varchar(32);not null;default:pending" json:"status"`

	// TaskID is the server-tasks ID the worker created when dispatching
	// the SSH script. Nil until the worker picks the row up. Frontend
	// uses it to subscribe to live log streaming.
	TaskID *string `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`

	// CommitSHA + CommitMsg only meaningful for git sources.
	CommitSHA *string `gorm:"column:commit_sha;type:varchar(64)" json:"commit_sha,omitempty"`
	CommitMsg *string `gorm:"column:commit_msg;type:text" json:"commit_msg,omitempty"`

	// ImageRef is the image reference (e.g. "nginx:1.27" or
	// "launch/project-app:<sha>") that this deploy ran. Useful for
	// rollback ("redeploy this image").
	ImageRef *string `gorm:"column:image_ref;type:varchar(512)" json:"image_ref,omitempty"`

	// LogPath is the on-server path to the script output captured during
	// the run. Kept for legacy rows; new rows leave it nil and rely on
	// TaskID + the task-logs websocket.
	LogPath *string `gorm:"column:log_path;type:varchar(512)" json:"log_path,omitempty"`

	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null" json:"started_at,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	Error      *string    `gorm:"type:text" json:"error,omitempty"`
}

func (Deployment) TableName() string { return "docker_deployments" }
