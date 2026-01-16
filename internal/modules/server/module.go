package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/handlers"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/providers"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/sshkey"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

// Ensure Module implements all required interfaces
var (
	_ app.Module           = (*Module)(nil)
	_ app.RouteRegistrar   = (*Module)(nil)
	_ app.WebhookRegistrar = (*Module)(nil)
	_ app.JobRegistrar     = (*Module)(nil)
)

// Module represents the server module
type Module struct {
	db                     *gorm.DB
	queueClient            *queue.Client
	ws                     *websocket.Hub
	dispatcher             *taskrunner.Dispatcher
	logger                 *zerolog.Logger
	handler                *handlers.Handler
	webhookHandler         *handlers.TaskWebhookHandler
	provisionScriptHandler *handlers.ProvisionScriptHandler
	repo                   *repositories.Repository
	service                *services.Service
}

// NewModule creates a new server module instance
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger, webhookSecretKey string) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, queueClient, ws, dispatcher, logger)

	// Create task runner deps for handlers
	taskRunnerDeps := &tasks.TaskRunnerDeps{
		DB:         db,
		Queue:      queueClient,
		Dispatcher: dispatcher,
		Logger:     logger,
	}

	handler := handlers.NewHandler(service, taskRunnerDeps)
	webhookHandler := handlers.NewTaskWebhookHandler(repo, webhookSecretKey, queueClient, logger)
	provisionScriptHandler := handlers.NewProvisionScriptHandler(repo, service)

	return &Module{
		db:                     db,
		queueClient:            queueClient,
		ws:                     ws,
		dispatcher:             dispatcher,
		logger:                 logger,
		handler:                handler,
		webhookHandler:         webhookHandler,
		provisionScriptHandler: provisionScriptHandler,
		repo:                   repo,
		service:                service,
	}
}

// NewModuleFromContext creates a new server module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(
		ctx.DB,
		ctx.Queue,
		ctx.WebSocket,
		ctx.Dispatcher,
		ctx.Logger,
		ctx.Config.App.Key,
	)
}

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "server"
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	m.webhookHandler.RegisterRoutes(router)
	m.provisionScriptHandler.RegisterRoutes(router)
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	providerFactory := providers.NewFactory(sshkey.NewGenerator())

	jobContext := jobs.NewJobContext(m.db, m.repo, m.logger, m.ws, m.dispatcher, providerFactory, m.queueClient)
	jobs.SetJobContext(jobContext)

	jobs.RegisterHandlers(mux)
}

// GetWebhookHandler returns the webhook handler for generating callback URLs
func (m *Module) GetWebhookHandler() *handlers.TaskWebhookHandler {
	return m.webhookHandler
}

// Repository returns the server repository
func (m *Module) Repository() *repositories.Repository {
	return m.repo
}

// SetSiteCounter sets the site counter for cross-module queries
func (m *Module) SetSiteCounter(counter handlers.SiteCounter) {
	m.handler.SetSiteCounter(counter)
}
