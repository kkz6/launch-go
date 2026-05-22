package models

import (
	"time"

	dockertypes "github.com/kkz6/launch-go/internal/modules/docker/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Deployment is a single deploy attempt for an application or compose
// stack. Polymorphic via (target_type, target_id) so the deploy history
// table is shared across workload kinds. Read paths always join on
// (target_type, target_id) — never on target_id alone — so cross-kind
// collisions are impossible.
type Deployment struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped

	// TargetType is "application" or "compose". Required to interpret
	// TargetID. Databases don't deploy — they restart — so they have a
	// separate event table (slice 2j).
	TargetType string `gorm:"column:target_type;type:varchar(32);not null;index:idx_docker_deployments_target" json:"target_type"`
	TargetID   string `gorm:"column:target_id;type:char(26);not null;index:idx_docker_deployments_target" json:"target_id"`

	Status dockertypes.DeploymentStatus `gorm:"type:varchar(32);not null;default:pending" json:"status"`

	// CommitSHA + CommitMsg only meaningful for git sources.
	CommitSHA *string `gorm:"column:commit_sha;type:varchar(64)" json:"commit_sha,omitempty"`
	CommitMsg *string `gorm:"column:commit_msg;type:text" json:"commit_msg,omitempty"`

	// ImageRef is the image reference (e.g. "nginx:1.27" or
	// "launch/project-app:<sha>") that this deploy ran. Useful for
	// rollback ("redeploy this image").
	ImageRef *string `gorm:"column:image_ref;type:varchar(512)" json:"image_ref,omitempty"`

	// LogPath is the on-server path to the script output captured during
	// the run. Phase 2 stores the tail of stdout in Error on failure;
	// streaming-log fetch lands in slice 2d.
	LogPath *string `gorm:"column:log_path;type:varchar(512)" json:"log_path,omitempty"`

	StartedAt  *time.Time `gorm:"column:started_at;type:timestamp null" json:"started_at,omitempty"`
	FinishedAt *time.Time `gorm:"column:finished_at;type:timestamp null" json:"finished_at,omitempty"`
	Error      *string    `gorm:"type:text" json:"error,omitempty"`
}

func (Deployment) TableName() string { return "docker_deployments" }
