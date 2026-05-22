package models

import (
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Project groups docker workloads (applications, compose stacks, databases)
// running on a server. The (server_id, name) pair is unique among
// non-soft-deleted rows.
type Project struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.ServerScoped
	basemodels.SoftDeleteModel
	Name        string  `gorm:"type:varchar(255);not null" json:"name"`
	Description *string `gorm:"type:varchar(500)" json:"description,omitempty"`

	// Eager-loaded counts (populated by the repository, not stored).
	ApplicationsCount int64 `gorm:"-" json:"applications_count"`
	ComposesCount     int64 `gorm:"-" json:"composes_count"`
	DatabasesCount    int64 `gorm:"-" json:"databases_count"`
}

func (Project) TableName() string { return "docker_projects" }
