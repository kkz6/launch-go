package site

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/queue"
	"github.com/kkz6/launch-go/internal/websocket"
)

type Module struct {
	db     *gorm.DB
	queue  *queue.Client
	ws     *websocket.Hub
	logger *zerolog.Logger
}

func NewModule(db *gorm.DB, queueClient *queue.Client, ws *websocket.Hub, logger *zerolog.Logger) *Module {
	return &Module{
		db:     db,
		queue:  queueClient,
		ws:     ws,
		logger: logger,
	}
}

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	sites := router.Group("/sites", authMiddleware, middleware.TeamScope())

	// Site CRUD
	sites.Get("/", m.List)
	sites.Post("/", m.Create)
	sites.Get("/:id", m.Show)
	sites.Put("/:id", m.Update)
	sites.Delete("/:id", m.Delete)

	// Deployments
	sites.Post("/:id/deploy", m.Deploy)
	sites.Get("/:id/deployments", m.ListDeployments)
	sites.Get("/:id/deployments/:deploymentId", m.ShowDeployment)
	sites.Post("/:id/rollback/:releaseId", m.Rollback)

	// Environment
	sites.Get("/:id/environment", m.GetEnvironment)
	sites.Put("/:id/environment", m.UpdateEnvironment)

	// SSL
	sites.Post("/:id/ssl", m.EnableSSL)
	sites.Delete("/:id/ssl", m.DisableSSL)
}

func (m *Module) List(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Sites retrieved", []interface{}{})
}

func (m *Module) Create(c *fiber.Ctx) error {
	// TODO: Implement
	return response.Created(c, "Site created", nil)
}

func (m *Module) Show(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Site retrieved", nil)
}

func (m *Module) Update(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Site updated", nil)
}

func (m *Module) Delete(c *fiber.Ctx) error {
	// TODO: Implement
	return response.NoContent(c)
}

func (m *Module) Deploy(c *fiber.Ctx) error {
	// TODO: Implement deployment trigger
	return response.OK(c, "Deployment started", nil)
}

func (m *Module) ListDeployments(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Deployments retrieved", []interface{}{})
}

func (m *Module) ShowDeployment(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Deployment retrieved", nil)
}

func (m *Module) Rollback(c *fiber.Ctx) error {
	// TODO: Implement rollback
	return response.OK(c, "Rollback started", nil)
}

func (m *Module) GetEnvironment(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Environment retrieved", nil)
}

func (m *Module) UpdateEnvironment(c *fiber.Ctx) error {
	// TODO: Implement
	return response.OK(c, "Environment updated", nil)
}

func (m *Module) EnableSSL(c *fiber.Ctx) error {
	// TODO: Implement SSL provisioning
	return response.OK(c, "SSL enabled", nil)
}

func (m *Module) DisableSSL(c *fiber.Ctx) error {
	// TODO: Implement SSL removal
	return response.OK(c, "SSL disabled", nil)
}
