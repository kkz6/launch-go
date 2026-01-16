package services

import (
	"strings"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	databaseservices "github.com/kkz6/launch-go/internal/modules/database/services"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	serverservices "github.com/kkz6/launch-go/internal/modules/server/services"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// ServiceDeps holds all dependencies needed for site services
type ServiceDeps struct {
	DB             *gorm.DB
	Logger         *zerolog.Logger
	Queue          *queue.Client
	WebSocket      *websocket.Hub
	TaskRunnerDeps *servertasks.TaskRunnerDeps
	Repos          *repositories.Registry

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

// CrossModuleDeps holds dependencies from other modules
type CrossModuleDeps struct {
	ServerRepo      *serverrepos.Repository
	ServerService   *serverservices.Service
	DatabaseService *databaseservices.Service
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

// SetCrossModuleDeps wires dependencies from other modules
func (r *ServiceRegistry) SetCrossModuleDeps(deps *CrossModuleDeps) {
	if deps.ServerRepo != nil {
		r.site.SetServerRepository(deps.ServerRepo)
	}
	if deps.ServerService != nil {
		r.site.SetServerService(deps.ServerService)
	}
	if deps.DatabaseService != nil {
		r.site.SetDatabaseService(deps.DatabaseService)
	}
}

// BaseService provides common service dependencies
type BaseService struct {
	service.Base
	deps  *ServiceDeps
	repos *repositories.Registry
}

// NewBaseService creates a new base service from ServiceDeps
func NewBaseService(deps *ServiceDeps) *BaseService {
	return &BaseService{
		Base:  service.NewBase(deps.Queue, deps.WebSocket, deps.Logger),
		deps:  deps,
		repos: deps.Repos,
	}
}

// Repos returns the repository registry
func (s *BaseService) Repos() *repositories.Registry {
	return s.repos
}

// ServiceDeps returns the service dependencies
func (s *BaseService) ServiceDeps() *ServiceDeps {
	return s.deps
}

// Services returns the service registry for accessing other services
func (s *BaseService) Services() *ServiceRegistry {
	return s.deps.registry
}

// Helper functions

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
