package repositories

import (
	"gorm.io/gorm"
)

// Registry holds all git module repositories
type Registry struct {
	sourceControl     *SourceControlRepository
	sourceControlRepo *SourceControlRepoRepository
}

// NewRegistry creates all repositories
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		sourceControl:     NewSourceControlRepository(db),
		sourceControlRepo: NewSourceControlRepoRepository(db),
	}
}

// SourceControl returns the source control repository
func (r *Registry) SourceControl() *SourceControlRepository { return r.sourceControl }

// SourceControlRepo returns the source control repo repository
func (r *Registry) SourceControlRepo() *SourceControlRepoRepository { return r.sourceControlRepo }
