package models

import (
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Deployment represents a site deployment
type Deployment struct {
	basemodels.BaseModel
	SiteID     string                 `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	UserID     *string                `gorm:"column:user_id;type:char(26);index" json:"user_id,omitempty"`
	TaskID     *string                `gorm:"column:task_id;type:char(26);index" json:"task_id,omitempty"`
	Status     enums.DeploymentStatus `gorm:"type:varchar(255);not null" json:"status"`
	GitHash    *string                `gorm:"column:git_hash;type:varchar(255)" json:"git_hash,omitempty"`
	CommitData basemodels.JSONMap     `gorm:"column:commit_data;type:json" json:"commit_data,omitempty"`
	VcsData    basemodels.JSONMap     `gorm:"column:vcs_data;type:json" json:"vcs_data,omitempty"`

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
