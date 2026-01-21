// Package jobs provides infrastructure for async job handling with asynq.
package jobs

// ModuleContext provides a generic job context with typed repository access.
// This eliminates the repeated JobContext definitions across modules by providing
// a standard pattern for embedding Base and adding module-specific repositories.
//
// Type parameter R represents the module's repository type (e.g., *repositories.Registry
// or a contracts.RepositoryRegistry interface).
//
// Example usage:
//
//	// Direct usage as a type alias
//	type JobContext = jobs.ModuleContext[*repositories.Registry]
//
//	// Embedding for extension with additional fields
//	type JobContext struct {
//	    *jobs.ModuleContext[*repositories.Registry]
//	    TaskRunnerDeps *servertasks.TaskRunnerDeps
//	}
//
//	// Creating a context
//	func NewJobContext(deps jobs.BaseDeps, repos *repositories.Registry) *JobContext {
//	    return jobs.NewModuleContext(deps, repos)
//	}
//
//	// Accessing repos in a job
//	func (j *SomeJob) Handle(ctx context.Context) error {
//	    site, err := j.Ctx.Repos().Sites().FindByID(ctx, siteID)
//	    // ...
//	}
type ModuleContext[R any] struct {
	Base
	repos R
}

// NewModuleContext creates a new ModuleContext with the given dependencies and repositories.
//
// Example:
//
//	ctx := jobs.NewModuleContext(deps, repos)
func NewModuleContext[R any](deps BaseDeps, repos R) *ModuleContext[R] {
	return &ModuleContext[R]{
		Base:  NewBase(deps),
		repos: repos,
	}
}

// Repos returns the module-specific repository registry or interface.
// This provides type-safe access to the repositories configured for this module.
//
// Example:
//
//	// With a registry struct
//	site, err := ctx.Repos().Sites().FindByID(ctx, siteID)
//
//	// With an interface
//	servers := ctx.Repos().Servers()
func (c *ModuleContext[R]) Repos() R {
	return c.repos
}

// SetRepos updates the repository registry.
// This is useful for testing when you need to inject mock repositories.
func (c *ModuleContext[R]) SetRepos(repos R) {
	c.repos = repos
}
