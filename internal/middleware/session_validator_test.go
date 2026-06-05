package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mintAccessTokenWithSession signs an HS256 access token carrying a session_id
// claim, mirroring how AuthService.generateAccessToken binds tokens to sessions.
func mintAccessTokenWithSession(t *testing.T, secret, sub, sessionID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":        sub,
		"type":       "access",
		"session_id": sessionID,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// validatorAllowing reports every session except those in the revoked set as live.
func validatorAllowing(revoked ...string) SessionValidator {
	gone := make(map[string]bool, len(revoked))
	for _, id := range revoked {
		gone[id] = true
	}
	return func(sessionID string) bool { return !gone[sessionID] }
}

// TestAuth_RevokedSessionRejected verifies that an access token whose backing
// session has been revoked (e.g. after logout) no longer authenticates, even
// though the JWT signature and expiry are still valid.
func TestAuth_RevokedSessionRejected(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(nil)
	InitSessionValidator(validatorAllowing("revoked-session"))
	t.Cleanup(func() { InitSessionValidator(nil) })

	app := fiber.New()
	app.Get("/guarded", Auth(secret, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+mintAccessTokenWithSession(t, secret, "u2", "revoked-session"))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// TestAuth_LiveSessionAllowed verifies a token whose session is still live
// authenticates normally.
func TestAuth_LiveSessionAllowed(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(nil)
	InitSessionValidator(validatorAllowing("revoked-session"))
	t.Cleanup(func() { InitSessionValidator(nil) })

	app := fiber.New()
	app.Get("/guarded", Auth(secret, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+mintAccessTokenWithSession(t, secret, "u2", "live-session"))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestAuth_SessionlessTokenUnaffected verifies tokens without a session_id
// (e.g. PAT-exchanged or impersonation tokens that don't bind a session) are
// not subjected to the session check.
func TestAuth_SessionlessTokenUnaffected(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(nil)
	// A validator that rejects everything must not affect a sessionless token.
	InitSessionValidator(func(string) bool { return false })
	t.Cleanup(func() { InitSessionValidator(nil) })

	app := fiber.New()
	app.Get("/guarded", Auth(secret, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+mintAccessToken(t, secret, "u2"))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// TestAuth_NoValidatorFailsOpen verifies that when no session validator is
// wired (tests / contexts without the auth repo), session-bound tokens behave
// exactly as before — preserving backward compatibility.
func TestAuth_NoValidatorFailsOpen(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(nil)
	InitSessionValidator(nil)

	app := fiber.New()
	app.Get("/guarded", Auth(secret, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+mintAccessTokenWithSession(t, secret, "u2", "any-session"))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
