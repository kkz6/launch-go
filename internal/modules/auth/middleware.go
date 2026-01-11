package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// AuthMiddleware validates JWT tokens and sets user context
func AuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Missing authorization header")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return response.Unauthorized(c, "Invalid authorization format")
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}

			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Invalid token")
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return response.Unauthorized(c, "Invalid token claims")
		}

		// Check token type
		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return response.Unauthorized(c, "Invalid token type")
		}

		// Set user info in context
		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		if teamID, ok := claims["team_id"].(string); ok {
			c.Locals("teamID", teamID)
		}

		return c.Next()
	}
}

// TwoFactorMiddleware checks if 2FA is required and verified
func TwoFactorMiddleware(service *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next() // No user authenticated, let auth middleware handle it
		}

		// Check if user has 2FA enabled
		has2FA, err := service.HasTwoFactorEnabled(c.Context(), userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check 2FA status")
		}

		if !has2FA {
			return c.Next() // 2FA not enabled, proceed
		}

		// Check if 2FA is verified in this session (via custom header or cookie)
		twoFactorVerified := c.Get("X-Two-Factor-Verified")
		if twoFactorVerified == "true" {
			return c.Next()
		}

		// Return 2FA required response
		return c.Status(fiber.StatusLocked).JSON(fiber.Map{
			"success":             false,
			"message":             "Two-factor authentication required",
			"two_factor_required": true,
		})
	}
}

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
func TeamMemberMiddleware(service *Service) fiber.Handler {
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

		isMember, err := service.repo.IsTeamMember(c.Context(), teamID, userID)
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
func TeamOwnerMiddleware(service *Service) fiber.Handler {
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
func TeamAdminMiddleware(service *Service) fiber.Handler {
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
		member, err := service.repo.GetTeamMember(c.Context(), teamID, userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to check membership")
		}

		if member == nil || member.Role != TeamRoleAdmin.String() {
			return response.Forbidden(c, "Only team admins can perform this action")
		}

		return c.Next()
	}
}

// EmailVerifiedMiddleware checks if the user's email is verified
func EmailVerifiedMiddleware(service *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "User not authenticated")
		}

		user, err := service.GetUser(c.Context(), userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to get user")
		}

		if user == nil {
			return response.NotFound(c, "User not found")
		}

		if !user.HasVerifiedEmail() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":               false,
				"message":               "Email verification required",
				"email_verified":        false,
				"requires_verification": true,
			})
		}

		return c.Next()
	}
}

// OptionalAuthMiddleware attempts to authenticate but allows anonymous access
func OptionalAuthMiddleware(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next() // No auth header, continue as anonymous
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.Next() // Invalid format, continue as anonymous
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}

			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Next() // Invalid token, continue as anonymous
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Next()
		}

		// Check token type
		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "access" {
			return c.Next()
		}

		// Set user info in context
		c.Locals("userID", claims["sub"])
		c.Locals("email", claims["email"])

		if teamID, ok := claims["team_id"].(string); ok {
			c.Locals("teamID", teamID)
		}

		return c.Next()
	}
}

// RateLimitMiddleware provides rate limiting based on IP or user
func RateLimitMiddleware(maxRequests int, windowSeconds int) fiber.Handler {
	// This is a simple placeholder - in production you would use Redis
	// or another distributed store for rate limiting
	return func(c *fiber.Ctx) error {
		// For now, just pass through
		// TODO: Implement proper rate limiting with Redis
		return c.Next()
	}
}
