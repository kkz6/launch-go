package models

// SourceControlRepository represents a repository synced from a git provider
type SourceControlRepository struct {
	ID              uint64  `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceControlID string  `gorm:"column:source_control_id;type:char(26);not null;index" json:"source_control_id"`
	Name            string  `gorm:"type:varchar(255);not null" json:"name"`
	FullName        string  `gorm:"column:full_name;type:varchar(255);not null" json:"full_name"`
	Public          bool    `gorm:"default:false" json:"public"`
	SSHURL          string  `gorm:"column:ssh_url;type:varchar(255);not null" json:"ssh_url"`
	DefaultBranch   string  `gorm:"column:default_branch;type:varchar(255);not null" json:"default_branch"`
	HTMLURL         *string `gorm:"column:html_url;type:varchar(255)" json:"html_url,omitempty"`
	AdditionalData  *string `gorm:"column:additional_data;type:json" json:"additional_data,omitempty"`

	// Relations
	SourceControl *SourceControl `gorm:"foreignKey:SourceControlID;references:ID" json:"source_control,omitempty"`
}

// TableName returns the table name for SourceControlRepository
func (SourceControlRepository) TableName() string {
	return "source_control_repositories"
}

// GetDefaultBranchOrMain returns the default branch or "main" if empty
func (r *SourceControlRepository) GetDefaultBranchOrMain() string {
	if r.DefaultBranch == "" {
		return "main"
	}

	return r.DefaultBranch
}

// GetHTMLURL returns the HTML URL or empty string if nil
func (r *SourceControlRepository) GetHTMLURL() string {
	if r.HTMLURL == nil {
		return ""
	}

	return *r.HTMLURL
}
