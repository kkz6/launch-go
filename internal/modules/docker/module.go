package docker

import (
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/docker/handlers"
	"github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/modules/docker/repositories"
	"github.com/kkz6/launch-go/internal/modules/docker/services"
	dockertasks "github.com/kkz6/launch-go/internal/modules/docker/tasks"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

const ModuleName = "docker"

// Ensure Module implements all required interfaces
var (
	_ app.Module                = (*Module)(nil)
	_ app.RouteRegistrar        = (*Module)(nil)
	_ app.WebhookRegistrar      = (*Module)(nil)
	_ app.JobRegistrar          = (*Module)(nil)
	_ app.TaskCallbackRegistrar = (*Module)(nil)
)

// Module represents the docker module
type Module struct {
	app.Base
	repos   *repositories.Registry
	service *services.DockerService
}

// NewModule creates a new docker module using the builder
func NewModule(b *app.Builder) *Module {
	deps := b.Deps()
	repos := repositories.NewRegistry(deps.DB)
	svc := services.NewDockerService(services.ServiceDeps{
		Dependencies: deps.ServiceDeps(),
		Repos:        repos,
	})

	return &Module{
		Base:    app.NewBase(ModuleName, b),
		repos:   repos,
		service: svc,
	}
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	webhookHandler := handlers.NewDockerWebhookHandler(m.service, m.repos.DockerService())
	router.Post("/webhooks/docker/:serviceId/:token", webhookHandler.Deploy)
}

// RegisterJobs registers background job handlers (implements app.JobRegistrar)
func (m *Module) RegisterJobs(mux *asynq.ServeMux) {
	jobs.Register(mux, m.Deps(), m.repos)
}

// RegisterTaskCallbacks registers task callback handlers (implements app.TaskCallbackRegistrar)
func (m *Module) RegisterTaskCallbacks() {
	dockertasks.RegisterTaskCallbacks()
}
