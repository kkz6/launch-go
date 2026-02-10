package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// TeamService defines the interface for team-related operations needed by middlewares
type TeamService interface {
	GetTeam(ctx context.Context, teamID string) (TeamInfo, error)
	IsTeamMember(ctx context.Context, teamID, userID string) (bool, error)
	GetTeamMember(ctx context.Context, teamID, userID string) (TeamMemberInfo, error)
}

// TeamInfo represents minimal team information needed by middlewares
type TeamInfo interface {
	GetUserID() string
}

// TeamMemberInfo represents minimal team member information needed by middlewares
type TeamMemberInfo interface {
	GetRole() string
	IsAdmin() bool
}

// TeamMember checks if the user is a member of the specified team
func TeamMember(service TeamService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			teamID, _ = c.Locals("teamID").(string)
		}

		if teamID == "" {
			return fiberctx.RespondForbidden(c, "Team context required")
		}

		isMember, err := service.IsTeamMember(c.Context(), teamID, userID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to check team membership")
		}

		if !isMember {
			if activity.IsInitialized() {
				activity.Activity().
					WithContext(c.Context()).
					UseLog("security").
					WithEvent("authorization_failed").
					CausedByUser(userID).
					WithProperties(map[string]any{
						"team_id":    teamID,
						"path":       c.Path(),
						"method":     c.Method(),
						"ip_address": c.IP(),
					}).
					Log("Team membership authorization failed")
			}
			return fiberctx.RespondForbidden(c, "You are not a member of this team")
		}

		return c.Next()
	}
}

// TeamOwner checks if the user is the owner of the specified team
func TeamOwner(service TeamService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			return fiberctx.RespondForbidden(c, "Team ID required")
		}

		team, err := service.GetTeam(c.Context(), teamID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to get team")
		}

		if team == nil {
			return fiberctx.RespondNotFound(c, "Team not found")
		}

		if team.GetUserID() != userID {
			if activity.IsInitialized() {
				activity.Activity().
					WithContext(c.Context()).
					UseLog("security").
					WithEvent("authorization_failed").
					CausedByUser(userID).
					WithProperties(map[string]any{
						"team_id":       teamID,
						"required_role": "owner",
						"path":          c.Path(),
						"method":        c.Method(),
						"ip_address":    c.IP(),
					}).
					Log("Insufficient role for requested action")
			}
			return fiberctx.RespondForbidden(c, "Only team owner can perform this action")
		}

		return c.Next()
	}
}

// TeamAdmin checks if the user is an admin or owner of the specified team
func TeamAdmin(service TeamService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "User not authenticated")
		}

		teamID := c.Params("teamId")
		if teamID == "" {
			return fiberctx.RespondForbidden(c, "Team ID required")
		}

		team, err := service.GetTeam(c.Context(), teamID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to get team")
		}

		if team == nil {
			return fiberctx.RespondNotFound(c, "Team not found")
		}

		if team.GetUserID() == userID {
			return c.Next()
		}

		member, err := service.GetTeamMember(c.Context(), teamID, userID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to check membership")
		}

		if member == nil || !member.IsAdmin() {
			if activity.IsInitialized() {
				currentRole := ""
				if member != nil {
					currentRole = member.GetRole()
				}
				activity.Activity().
					WithContext(c.Context()).
					UseLog("security").
					WithEvent("authorization_failed").
					CausedByUser(userID).
					WithProperties(map[string]any{
						"team_id":       teamID,
						"current_role":  currentRole,
						"required_role": "admin",
						"path":          c.Path(),
						"method":        c.Method(),
						"ip_address":    c.IP(),
					}).
					Log("Insufficient role for requested action")
			}
			return fiberctx.RespondForbidden(c, "Only team admins can perform this action")
		}

		return c.Next()
	}
}
