package database

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/handlers"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Ensure Module implements required interfaces
var (
	_ app.Module         = (*Module)(nil)
	_ app.RouteRegistrar = (*Module)(nil)
	_ app.JobRegistrar   = (*Module)(nil)
)

// Module represents the database module
type Module struct {
	db         *gorm.DB
	ws         *websocket.Hub
	dispatcher taskrunner.TaskDispatcher
	logger     *zerolog.Logger
	handler    *handlers.Handler
	repo       *repositories.Repository
	queue      *queue.Client
}

// NewModule creates a new database module
func NewModule(db *gorm.DB, serverRepo services.ServerRepository, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	return NewModuleWithDispatcher(db, serverRepo, queueClient, ws, nil, logger)
}

// NewModuleWithDispatcher creates a new database module with a dispatcher for jobs
func NewModuleWithDispatcher(db *gorm.DB, serverRepo services.ServerRepository, queueClient *queue.Client, ws *websocket.Hub, dispatcher taskrunner.TaskDispatcher, logger *zerolog.Logger) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, serverRepo, queueClient, ws, logger)
	handler := handlers.NewHandler(service)

	// Initialize the job context for this module
	jobContext := jobs.NewJobContext(db, repo, logger, ws, dispatcher, queueClient)
	jobs.SetJobContext(jobContext)

	return &Module{
		db:         db,
		ws:         ws,
		dispatcher: dispatcher,
		logger:     logger,
		handler:    handler,
		repo:       repo,
		queue:      queueClient,
	}
}

// NewModuleFromContext creates a new database module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModuleWithDispatcher(
		ctx.DB,
		nil, // serverRepo - will be injected if needed
		ctx.Queue,
		ctx.WebSocket,
		ctx.Dispatcher,
		ctx.Logger,
	)
}

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "database"
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	// Initialize the job context for this module
	jobContext := jobs.NewJobContext(m.db, m.repo, m.logger, m.ws, m.dispatcher, m.queue)
	jobs.SetJobContext(jobContext)

	// Create kernel and register jobs
	kernel := pkgjobs.NewKernel(m.db, m.logger, m.ws)
	kernel.RegisterModule(jobs.Register)
	kernel.Boot(mux)
}

// RegisterJobsWithRegistry registers jobs using the Handler interface pattern.
// This is called by the kernel to register jobs.
func (m *Module) RegisterJobsWithRegistry(r *pkgjobs.Registry) {
	jobs.Register(r)
}
