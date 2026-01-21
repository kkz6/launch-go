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

// ModuleDepsWithRegistry provides service dependencies with a service registry.
// R is the repository registry type for the module.
// S is the service registry type for the module.
//
// Usage:
//
//	type ServiceDeps struct {
//	    service.ModuleDepsWithRegistry[*repositories.Registry, *ServiceRegistry]
//	}
type ModuleDepsWithRegistry[R any, S any] struct {
	ModuleDeps[R]
	Registry S
}

// ModuleBaseWithRegistry provides a generic base service for modules with service registry access.
// R is the repository registry type for the module.
// S is the service registry type for the module.
//
// This eliminates the common pattern of duplicating accessor methods like:
//
//	func (s *BaseService) Repos() *repositories.Registry { return s.repos }
//	func (s *BaseService) Services() *ServiceRegistry { return s.deps.registry }
//
// Usage:
//
//	type MyService struct {
//	    *service.ModuleBaseWithRegistry[*repositories.Registry, *ServiceRegistry]
//	}
//
//	func NewMyService(deps *ServiceDeps) *MyService {
//	    return &MyService{
//	        ModuleBaseWithRegistry: service.NewModuleBaseWithRegistry(&deps.ModuleDepsWithRegistry),
//	    }
//	}
type ModuleBaseWithRegistry[R any, S any] struct {
	Base
	deps     *ModuleDepsWithRegistry[R, S]
	repos    R
	registry S
}

// NewModuleBaseWithRegistry creates a new module base service with registry access
func NewModuleBaseWithRegistry[R any, S any](deps *ModuleDepsWithRegistry[R, S]) *ModuleBaseWithRegistry[R, S] {
	return &ModuleBaseWithRegistry[R, S]{
		Base:     NewBaseFromDeps(deps.Dependencies),
		deps:     deps,
		repos:    deps.Repos,
		registry: deps.Registry,
	}
}

// Repos returns the repository registry
func (s *ModuleBaseWithRegistry[R, S]) Repos() R {
	return s.repos
}

// Services returns the service registry for cross-service access
func (s *ModuleBaseWithRegistry[R, S]) Services() S {
	return s.registry
}

// ModuleDepsPtr returns the module dependencies pointer
func (s *ModuleBaseWithRegistry[R, S]) ModuleDepsPtr() *ModuleDepsWithRegistry[R, S] {
	return s.deps
}

// SetRegistry sets the service registry (used during initialization)
func (s *ModuleBaseWithRegistry[R, S]) SetRegistry(registry S) {
	s.registry = registry
	s.deps.Registry = registry
}
