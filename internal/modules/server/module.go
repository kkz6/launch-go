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
	"github.com/kkz6/launch-go/internal/pkg/app"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
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
	db             *gorm.DB
	queueClient    *queue.Client
	ws             *websocket.Hub
	dispatcher     *taskrunner.Dispatcher
	logger         *zerolog.Logger
	handler        *handlers.Handler
	webhookHandler *handlers.TaskWebhookHandler
	repo           *repositories.Repository
}

// NewModule creates a new server module instance
func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, dispatcher *taskrunner.Dispatcher, logger *zerolog.Logger, webhookSecretKey string) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, queueClient, ws, dispatcher, logger)
	handler := handlers.NewHandler(service)
	webhookHandler := handlers.NewTaskWebhookHandler(repo, webhookSecretKey)

	return &Module{
		db:             db,
		queueClient:    queueClient,
		ws:             ws,
		dispatcher:     dispatcher,
		logger:         logger,
		handler:        handler,
		webhookHandler: webhookHandler,
		repo:           repo,
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
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	// Create provider factory with key generator
	keyGen := &sshKeyGenerator{}
	providerFactory := providers.NewFactory(keyGen)

	// Set up job context
	jobContext := jobs.NewJobContext(m.db, m.repo, m.logger, m.ws, m.dispatcher, providerFactory)
	jobs.SetJobContext(jobContext)

	// Create kernel and register jobs
	kernel := pkgjobs.NewKernel(m.db, m.logger, m.ws)
	kernel.RegisterModule(jobs.Register)
	kernel.Boot(mux)
}

// sshKeyGenerator implements providers.KeyPairGenerator
type sshKeyGenerator struct{}

func (g *sshKeyGenerator) Generate() (*providers.KeyPair, error) {
	privateKey, publicKey, err := services.GenerateSSHKeyPair()
	if err != nil {
		return nil, err
	}
	return &providers.KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

func (g *sshKeyGenerator) GetPublicKey(privateKey string) (*providers.KeyPair, error) {
	// For now, just return the private key - public key extraction can be added later
	return &providers.KeyPair{
		PrivateKey: privateKey,
	}, nil
}

// GetWebhookHandler returns the webhook handler for generating callback URLs
func (m *Module) GetWebhookHandler() *handlers.TaskWebhookHandler {
	return m.webhookHandler
}

// Repository returns the server repository
func (m *Module) Repository() *repositories.Repository {
	return m.repo
}
