package services

import (
	"errors"
	"strings"

	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

var (
	ErrPendingDeployment         = fiberutil.Conflict("A deployment is already in progress")
	ErrRollbackNotSupported      = fiberutil.BadRequest("Rollback is only available for sites with zero downtime deployment enabled")
	ErrInvalidRollbackTarget     = fiberutil.BadRequest("Can only rollback to a finished deployment")
	ErrDeploymentNotBelongToSite = fiberutil.BadRequest("Target deployment does not belong to this site")
	ErrSourceControlNotConnected = fiberutil.BadRequest("Source control is not connected")
	ErrSiteNotInstalled          = fiberutil.BadRequest("Site is not installed")
	ErrBranchMismatch            = errors.New("branch mismatch")
	ErrInvalidDeployToken        = fiberutil.Unauthorized("Invalid deploy token")
)

// ServiceDeps holds all dependencies needed for site services.
// Embedding service.ModuleDeps provides common dependencies and repository access.
type ServiceDeps struct {
	service.ModuleDeps[*repositories.Registry]
	TaskRunnerDeps *servertasks.TaskRunnerDeps

	// Service registry - allows services to access other services
	registry *ServiceRegistry
}

// ServiceRegistry holds all services for cross-service access
type ServiceRegistry struct {
	site       *SiteService
	deployment *DeploymentService
	ssl        *SSLService
	queue      *QueueService
	command    *CommandService
	redirect   *RedirectService
	file       *FileService
}

// Site returns the site service
func (r *ServiceRegistry) Site() *SiteService { return r.site }

// Deployment returns the deployment service
func (r *ServiceRegistry) Deployment() *DeploymentService { return r.deployment }

// SSL returns the SSL service
func (r *ServiceRegistry) SSL() *SSLService { return r.ssl }

// Queue returns the queue service
func (r *ServiceRegistry) Queue() *QueueService { return r.queue }

// Command returns the command service
func (r *ServiceRegistry) Command() *CommandService { return r.command }

// Redirect returns the redirect service
func (r *ServiceRegistry) Redirect() *RedirectService { return r.redirect }

// File returns the file service
func (r *ServiceRegistry) File() *FileService { return r.file }

// CrossModuleDeps holds dependencies from other modules using interfaces
type CrossModuleDeps struct {
	ServerReader    contracts.ServerReader
	GitReader       contracts.GitReader
	CronCreator     contracts.CronCreator
	DatabaseManager contracts.DatabaseManager
	ProviderFactory *gitproviders.ProviderFactory
}

// NewServiceRegistry creates all services and wires them together
func NewServiceRegistry(deps *ServiceDeps) *ServiceRegistry {
	registry := &ServiceRegistry{}
	deps.registry = registry

	// Create all services
	registry.site = NewSiteService(deps)
	registry.deployment = NewDeploymentService(deps)
	registry.ssl = NewSSLService(deps)
	registry.queue = NewQueueService(deps)
	registry.command = NewCommandService(deps)
	registry.redirect = NewRedirectService(deps)
	registry.file = NewFileService(deps)

	return registry
}

// SetCrossModuleDeps wires dependencies from other modules using interfaces
func (r *ServiceRegistry) SetCrossModuleDeps(deps *CrossModuleDeps) {
	if deps.ServerReader != nil {
		r.site.SetServerReader(deps.ServerReader)
		r.file.SetServerReader(deps.ServerReader)
	}
	if deps.GitReader != nil {
		r.site.SetGitReader(deps.GitReader)
		r.deployment.SetGitReader(deps.GitReader)
	}
	if deps.CronCreator != nil {
		r.site.SetCronCreator(deps.CronCreator)
	}
	if deps.DatabaseManager != nil {
		r.site.SetDatabaseManager(deps.DatabaseManager)
	}
	if deps.ProviderFactory != nil {
		r.deployment.SetProviderFactory(deps.ProviderFactory)
	}
}

// BaseService provides common service dependencies for the site module.
// It wraps service.ModuleBase and adds site-specific functionality.
type BaseService struct {
	*service.ModuleBase[*repositories.Registry]
	serviceDeps *ServiceDeps
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		ModuleBase:  service.NewModuleBase(&deps.ModuleDeps),
		serviceDeps: deps,
	}
}

// ServiceDeps returns the service dependencies
func (s *BaseService) ServiceDeps() *ServiceDeps {
	return s.serviceDeps
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.serviceDeps.registry
}

// Helper functions

// stringToPtr converts a string to a *string, returning nil for empty strings.
func stringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	return s
}

func parseMultilineToSlice(s string) []string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}

// Update helper functions for building field maps

// addIfSet adds a value to the updates map if the pointer is not nil
func addIfSet[T any](updates map[string]any, key string, value *T) {
	if value != nil {
		updates[key] = *value
	}
}

// addHookIfSet adds a hook value with normalized line endings
func addHookIfSet(updates map[string]any, key string, value *string) {
	if value != nil {
		normalized := normalizeLineEndings(*value)
		updates[key] = &normalized
	}
}

// addSliceIfSet adds a multiline string as a slice
func addSliceIfSet(updates map[string]any, key string, value *string) {
	if value != nil {
		updates[key] = parseMultilineToSlice(*value)
	}
}
