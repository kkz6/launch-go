package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"gorm.io/gorm"
)

// serverProvisionedMiddleware holds the shared state for server provisioning checks
var serverProvisionedMiddleware struct {
	db *gorm.DB
}

// InitServerProvisionedMiddleware initializes the middleware with required dependencies.
// Call this during application bootstrap before routes are registered.
func InitServerProvisionedMiddleware(db *gorm.DB) {
	serverProvisionedMiddleware.db = db
}

// RequireProvisionedServer middleware ensures the server has status "running".
// Must be used after Auth and TeamScope middleware so that teamID is available in Locals.
//
// Usage:
//
//	servers.Get("/:id/services", middleware.RequireProvisionedServer(), handler.ListServices)
func RequireProvisionedServer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return fiberctx.RespondBadRequest(c, "Missing server ID")
		}

		if _, err := ulid.Parse(id); err != nil {
			return fiberctx.RespondBadRequest(c, "Invalid server ID format")
		}

		teamID, ok := c.Locals("teamID").(string)
		if !ok || teamID == "" {
			return fiberctx.Error(c, fiber.StatusBadRequest, "Team context required")
		}

		if serverProvisionedMiddleware.db == nil {
			return c.Next()
		}

		var status string
		err := serverProvisionedMiddleware.db.
			Table("servers").
			Select("status").
			Where("id = ? AND team_id = ?", id, teamID).
			Scan(&status).Error

		if err != nil || status == "" {
			return fiberctx.RespondForbidden(c, "Server not found")
		}

		if status != "running" {
			return fiberctx.RespondForbidden(c, "Server is not provisioned")
		}

		return c.Next()
	}
}
