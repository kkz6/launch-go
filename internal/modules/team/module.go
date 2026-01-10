package team

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

type Module struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func NewModule(db *gorm.DB, logger *zerolog.Logger) *Module {
	return &Module{
		db:     db,
		logger: logger,
	}
}

func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	teams := router.Group("/teams", authMiddleware, middleware.TeamScope())

	teams.Get("/", m.List)
	teams.Post("/", m.Create)
	teams.Get("/:id", m.Show)
	teams.Put("/:id", m.Update)
	teams.Delete("/:id", m.Delete)
	teams.Post("/:id/invite", m.Invite)
	teams.Delete("/:id/members/:userId", m.RemoveMember)
}

func (m *Module) List(c *fiber.Ctx) error {
	// TODO: Implement team listing
	return response.OK(c, "Teams retrieved", []interface{}{})
}

func (m *Module) Create(c *fiber.Ctx) error {
	// TODO: Implement team creation
	return response.Created(c, "Team created", nil)
}

func (m *Module) Show(c *fiber.Ctx) error {
	// TODO: Implement team show
	return response.OK(c, "Team retrieved", nil)
}

func (m *Module) Update(c *fiber.Ctx) error {
	// TODO: Implement team update
	return response.OK(c, "Team updated", nil)
}

func (m *Module) Delete(c *fiber.Ctx) error {
	// TODO: Implement team deletion
	return response.NoContent(c)
}

func (m *Module) Invite(c *fiber.Ctx) error {
	// TODO: Implement team invitation
	return response.Created(c, "Invitation sent", nil)
}

func (m *Module) RemoveMember(c *fiber.Ctx) error {
	// TODO: Implement member removal
	return response.NoContent(c)
}
