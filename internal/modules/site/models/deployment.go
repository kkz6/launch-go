package models

import (
	"time"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Deployment represents a site deployment
type Deployment struct {
	basemodels.BaseModel
	basemodels.SiteScoped
	basemodels.TeamScoped
	UserID         *string                    `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	TaskID         *string                    `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`
	Status         sitetypes.DeploymentStatus `gorm:"type:varchar(255);not null" json:"status"`
	GitHash        *string                    `gorm:"column:git_hash;type:varchar(255)" json:"git_hash,omitempty"`
	CommitData     dbtype.JSONMap             `gorm:"column:commit_data;type:json" json:"commit_data,omitempty"`
	VcsData        dbtype.JSONMap             `gorm:"column:vcs_data;type:json" json:"vcs_data,omitempty"`
	UserNotifiedAt *time.Time                 `gorm:"column:user_notified_at;type:timestamp null" json:"user_notified_at,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (Deployment) TableName() string {
	return "deployments"
}

// GetShortGitHash returns the first 7 characters of the git hash
func (d *Deployment) GetShortGitHash() string {
	if d.GitHash == nil {
		return ""
	}

	if len(*d.GitHash) < 7 {
		return *d.GitHash
	}

	return (*d.GitHash)[:7]
}

// IsRollback checks if the deployment is a rollback
func (d *Deployment) IsRollback() bool {
	_, hasRollbackFrom := d.CommitData["rollback_from"]
	_, hasRollbackTo := d.CommitData["rollback_to"]

	return hasRollbackFrom && hasRollbackTo
}

// CommitMessage returns the commit message from commit data
func (d *Deployment) CommitMessage() string {
	if msg, ok := d.CommitData["message"]; ok {
		if msgStr, ok := msg.(string); ok {
			return msgStr
		}
	}
	return ""
}

// CommitAuthor returns the commit author name from commit data
func (d *Deployment) CommitAuthor() string {
	if author, ok := d.CommitData["author_name"]; ok {
		if authorStr, ok := author.(string); ok {
			return authorStr
		}
	}
	return ""
}
