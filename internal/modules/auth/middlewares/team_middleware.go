package middlewares

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/enums"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TeamScopeMiddleware ensures a team context is present
func TeamScopeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Locals("teamID")
		if teamID == nil || teamID == "" {
			return response.Forbidden(c, "Team context required")
		}

		return c.Next()
	}
}

// TeamMemberMiddleware checks if the user is a member of the specified team
func TeamMemberMiddleware(service *services.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			teamID, _ = c.Locals("teamID").(string)
		}

		if teamID == "" {
			return response.Forbidden(c, "Team context required")
		}

		isMember, err := service.Repository().IsTeamMember(c.Context(), teamID, userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check team membership")
		}

		if !isMember {
			return response.Forbidden(c, "You are not a member of this team")
		}

		return c.Next()
	}
}

// TeamOwnerMiddleware checks if the user is the owner of the specified team
func TeamOwnerMiddleware(service *services.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			return response.Forbidden(c, "Team ID required")
		}

		team, err := service.GetTeam(c.Context(), teamID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to get team")
		}

		if team == nil {
			return response.NotFound(c, "Team not found")
		}

		if team.OwnerID != userID {
			return response.Forbidden(c, "Only team owner can perform this action")
		}

		return c.Next()
	}
}

// TeamAdminMiddleware checks if the user is an admin or owner of the specified team
func TeamAdminMiddleware(service *services.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			return response.Forbidden(c, "Team ID required")
		}

		team, err := service.GetTeam(c.Context(), teamID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to get team")
		}

		if team == nil {
			return response.NotFound(c, "Team not found")
		}

		// Check if owner
		if team.OwnerID == userID {
			return c.Next()
		}

		// Check if admin
		member, err := service.Repository().GetTeamMember(c.Context(), teamID, userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check membership")
		}

		if member == nil || member.Role != enums.TeamRoleAdmin.String() {
			return response.Forbidden(c, "Only team admins can perform this action")
		}

		return c.Next()
	}
}
