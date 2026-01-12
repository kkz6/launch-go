package models

import (
	"encoding/json"

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
	CommitData *string                `gorm:"column:commit_data;type:json" json:"commit_data,omitempty"`
	VcsData    *string                `gorm:"column:vcs_data;type:json" json:"vcs_data,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (d *Deployment) TableName() string {
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

// GetCommitData returns commit data as a map
func (d *Deployment) GetCommitData() map[string]interface{} {
	if d.CommitData == nil || *d.CommitData == "" || *d.CommitData == "null" {
		return map[string]interface{}{}
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*d.CommitData), &data); err != nil {
		return map[string]interface{}{}
	}

	return data
}

// SetCommitData sets commit data from a map
func (d *Deployment) SetCommitData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	str := string(jsonData)
	d.CommitData = &str

	return nil
}

// IsRollback checks if the deployment is a rollback
func (d *Deployment) IsRollback() bool {
	data := d.GetCommitData()
	_, hasRollbackFrom := data["rollback_from"]
	_, hasRollbackTo := data["rollback_to"]

	return hasRollbackFrom && hasRollbackTo
}
