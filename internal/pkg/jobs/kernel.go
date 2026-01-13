package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
)

// Kernel is the central job registration point.
// Similar to Laravel's App\Console\Kernel but for queue jobs.
//
// Usage:
//
//	kernel := jobs.NewKernel(db, logger, ws)
//
//	// Register modules using the Handler interface pattern
//	kernel.RegisterModule(serverjobs.Register)
//
//	// Register modules using the mux pattern (legacy)
//	kernel.RegisterMuxModule(databasejobs.RegisterHandlers)
//
//	kernel.Boot(mux)
type Kernel struct {
	registry   *Registry
	modules    []ModuleRegistrar
	muxModules []MuxModuleRegistrar
}

// ModuleRegistrar is a function that registers jobs using the Handler interface pattern
type ModuleRegistrar func(r *Registry)

// MuxModuleRegistrar is a function that registers jobs directly with the mux
// This supports modules that haven't been migrated to the Handler interface pattern yet
type MuxModuleRegistrar func(mux *asynq.ServeMux)

// NewKernel creates a new job kernel
func NewKernel(db *gorm.DB, logger *zerolog.Logger, ws Broadcaster) *Kernel {
	return &Kernel{
		registry:   NewRegistry(db, logger, ws),
		modules:    make([]ModuleRegistrar, 0),
		muxModules: make([]MuxModuleRegistrar, 0),
	}
}

// WithDispatcher sets the task dispatcher on the kernel's registry
func (k *Kernel) WithDispatcher(dispatcher taskrunner.TaskDispatcher) *Kernel {
	k.registry.dispatcher = dispatcher
	return k
}

// WithQueue sets the queue client on the kernel's registry
func (k *Kernel) WithQueue(q *queue.Client) *Kernel {
	k.registry.queue = q
	return k
}

// RegisterModule adds a module's job registrar (Handler interface pattern)
func (k *Kernel) RegisterModule(registrar ModuleRegistrar) {
	k.modules = append(k.modules, registrar)
}

// RegisterMuxModule adds a module's job registrar (direct mux pattern)
// Use this for modules that use HandleFunc directly
func (k *Kernel) RegisterMuxModule(registrar MuxModuleRegistrar) {
	k.muxModules = append(k.muxModules, registrar)
}

// Boot registers all jobs with the asynq mux
func (k *Kernel) Boot(mux *asynq.ServeMux) {
	// Register all Handler interface pattern modules
	for _, registrar := range k.modules {
		registrar(k.registry)
	}

	// Register handlers from Handler interface pattern
	k.registry.RegisterHandlers(mux)

	// Register all direct mux pattern modules
	for _, registrar := range k.muxModules {
		registrar(mux)
	}
}

// Registry returns the underlying registry
func (k *Kernel) Registry() *Registry {
	return k.registry
}
