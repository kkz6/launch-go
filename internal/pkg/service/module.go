package service

// ModuleDeps provides a generic service dependencies struct for modules.
// R is the repository registry type for the module.
//
// Usage:
//
//	type ServerServiceDeps = service.ModuleDeps[*repositories.Registry]
type ModuleDeps[R any] struct {
	Dependencies
	Repos R
}

// ModuleBase provides a generic base service for modules.
// R is the repository registry type for the module.
//
// Usage:
//
//	type MyService struct {
//	    *service.ModuleBase[*repositories.Registry]
//	}
type ModuleBase[R any] struct {
	Base
	deps  *ModuleDeps[R]
	repos R
}

// NewModuleBase creates a new module base service
func NewModuleBase[R any](deps *ModuleDeps[R]) *ModuleBase[R] {
	return &ModuleBase[R]{
		Base:  NewBaseFromDeps(deps.Dependencies),
		deps:  deps,
		repos: deps.Repos,
	}
}

// Repos returns the repository registry
func (s *ModuleBase[R]) Repos() R {
	return s.repos
}

// Deps returns the full dependencies
func (s *ModuleDeps[R]) Deps() *ModuleDeps[R] {
	return s
}
