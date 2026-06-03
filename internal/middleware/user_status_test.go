package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
)

// statusLoader returns suspended for "u1" and active for everyone else.
func statusLoader(userID string) authtypes.UserStatus {
	if userID == "u1" {
		return authtypes.UserStatusSuspended
	}
	return authtypes.UserStatusActive
}

// appWithStatusProbe builds an app whose guarded route reports the result of
// isUserSuspended directly: 403 when suspended, 200 otherwise.
func appWithStatusProbe(load UserStatusLoader, userID string) *fiber.App {
	InitUserStatus(load)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("userID", userID); return c.Next() })
	app.Get("/probe", func(c *fiber.Ctx) error {
		if isUserSuspended(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestIsUserSuspended_SuspendedForbidden(t *testing.T) {
	app := appWithStatusProbe(statusLoader, "u1")
	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestIsUserSuspended_ActiveAllowed(t *testing.T) {
	app := appWithStatusProbe(statusLoader, "u2")
	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestIsUserSuspended_NoLoaderAllowed(t *testing.T) {
	app := appWithStatusProbe(nil, "u1")
	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// mintAccessToken signs a minimal HS256 access token for the given subject.
func mintAccessToken(t *testing.T, secret, sub string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  sub,
		"type": "access",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// TestAuth_SuspendedUserRejected verifies the suspended check fires inside the
// real Auth() chokepoint. The JWT path does not touch the DB, so a nil db is
// safe here.
func TestAuth_SuspendedUserRejected(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(statusLoader)

	app := fiber.New()
	app.Get("/guarded", Auth(secret, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/guarded", nil)
	req.Header.Set("Authorization", "Bearer "+mintAccessToken(t, secret, "u1"))
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestAuth_ActiveUserAllowed(t *testing.T) {
	const secret = "test-secret"
	InitUserStatus(statusLoader)

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
