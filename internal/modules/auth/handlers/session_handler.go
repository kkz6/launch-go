package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// SessionHandler handles session management endpoints
type SessionHandler struct {
	sessionRepo contracts.SessionRepository
}

// NewSessionHandler creates a new SessionHandler
func NewSessionHandler(sessionRepo contracts.SessionRepository) *SessionHandler {
	return &SessionHandler{sessionRepo: sessionRepo}
}

type sessionResponse struct {
	ID         string    `json:"id"`
	Agent      agentInfo `json:"agent"`
	IPAddress  string    `json:"ip_address"`
	IsCurrent  bool      `json:"is_current_device"`
	LastActive string    `json:"last_active"`
}

type agentInfo struct {
	Browser   string `json:"browser"`
	IsDesktop bool   `json:"is_desktop"`
	Platform  string `json:"platform"`
}

// List returns all active sessions for the current user
func (h *SessionHandler) List(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	currentSessionID, _ := c.Locals("sessionID").(string)

	sessions, err := h.sessionRepo.GetByUser(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	results := make([]sessionResponse, len(sessions))
	for i, s := range sessions {
		var ua string
		if s.UserAgent != nil {
			ua = *s.UserAgent
		}
		var ip string
		if s.IPAddress != nil {
			ip = *s.IPAddress
		}

		results[i] = sessionResponse{
			ID:         s.ID,
			Agent:      parseUserAgent(ua),
			IPAddress:  ip,
			IsCurrent:  s.ID == currentSessionID,
			LastActive: time.Unix(int64(s.LastActivity), 0).UTC().Format(time.RFC3339),
		}
	}

	return fiberctx.OK(c, "Sessions retrieved", results)
}

// Revoke deletes a specific session
func (h *SessionHandler) Revoke(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return fiberctx.RespondBadRequest(c, "Session ID is required")
	}

	if err := h.sessionRepo.DeleteByUser(c.Context(), sessionID, userID); err != nil {
		return fiberctx.RespondNotFound(c, "Session not found")
	}

	return fiberctx.OK(c, "Session revoked", nil)
}

// RevokeOthers deletes all sessions except the current one
func (h *SessionHandler) RevokeOthers(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	currentSessionID, _ := c.Locals("sessionID").(string)
	if currentSessionID == "" {
		return fiberctx.RespondBadRequest(c, "Current session not found")
	}

	count, err := h.sessionRepo.DeleteAllByUserExcept(c.Context(), userID, currentSessionID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Other sessions revoked", fiber.Map{"revoked_count": count})
}

// parseUserAgent extracts browser, platform, and device type from a user agent string
func parseUserAgent(ua string) agentInfo {
	if ua == "" {
		return agentInfo{Browser: "Unknown", Platform: "Unknown", IsDesktop: true}
	}

	lower := strings.ToLower(ua)

	browser := "Unknown"
	switch {
	case strings.Contains(lower, "edg/") || strings.Contains(lower, "edge/"):
		browser = "Edge"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		browser = "Opera"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg"):
		browser = "Chrome"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	case strings.Contains(lower, "curl"):
		browser = "curl"
	}

	platform := "Unknown"
	switch {
	case strings.Contains(lower, "windows"):
		platform = "Windows"
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad"):
		platform = "iOS"
	case strings.Contains(lower, "android"):
		platform = "Android"
	case strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os"):
		platform = "macOS"
	case strings.Contains(lower, "linux"):
		platform = "Linux"
	}

	isDesktop := !strings.Contains(lower, "mobile") &&
		!strings.Contains(lower, "android") &&
		!strings.Contains(lower, "iphone")

	return agentInfo{
		Browser:   browser,
		IsDesktop: isDesktop,
		Platform:  platform,
	}
}
