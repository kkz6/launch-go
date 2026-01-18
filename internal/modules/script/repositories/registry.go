package repositories

import "gorm.io/gorm"

// Registry holds all script module repositories
type Registry struct {
	script    *ScriptRepository
	execution *ScriptExecutionRepository
}

// NewRegistry creates a new repository registry
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		script:    NewScriptRepository(db),
		execution: NewScriptExecutionRepository(db),
	}
}

// Script returns the script repository
func (r *Registry) Script() *ScriptRepository {
	return r.script
}

// Execution returns the execution repository
func (r *Registry) Execution() *ScriptExecutionRepository {
	return r.execution
}
