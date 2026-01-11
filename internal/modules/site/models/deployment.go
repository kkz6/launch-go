package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Deployment represents a site deployment
type Deployment struct {
	ID             string                 `gorm:"primaryKey;size:26" json:"id"`
	SiteID         string                 `gorm:"size:26;not null;index" json:"site_id"`
	UserID         *string                `gorm:"size:26;index" json:"user_id,omitempty"`
	TaskID         *string                `gorm:"size:26;index" json:"task_id,omitempty"`
	Status         enums.DeploymentStatus `gorm:"size:50;default:'pending'" json:"status"`
	GitHash        *string                `gorm:"size:40" json:"git_hash,omitempty"`
	CommitData     string                 `gorm:"type:json" json:"commit_data,omitempty"`
	VcsData        string                 `gorm:"type:json" json:"vcs_data,omitempty"`
	UserNotifiedAt *time.Time             `json:"user_notified_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (d *Deployment) TableName() string {
	return "deployments"
}

func (d *Deployment) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}

	return nil
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
	if d.CommitData == "" || d.CommitData == "null" {
		return map[string]interface{}{}
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(d.CommitData), &data); err != nil {
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

	d.CommitData = string(jsonData)

	return nil
}

// IsRollback checks if the deployment is a rollback
func (d *Deployment) IsRollback() bool {
	data := d.GetCommitData()
	_, hasRollbackFrom := data["rollback_from"]
	_, hasRollbackTo := data["rollback_to"]

	return hasRollbackFrom && hasRollbackTo
}
