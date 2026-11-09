package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// appWithImpersonation builds a Fiber app whose route is guarded by
// BlockImpersonationWrites. When readOnly is true an inline middleware sets the
// impersonationReadOnly local before the block runs, mirroring what
// setAuthContext does for a real impersonation token.
func appWithImpersonation(readOnly bool) *fiber.App {
	app := fiber.New()

	app.Use(func(c *fiber.Ctx) error {
		if readOnly {
			c.Locals("impersonationReadOnly", true)
		}

		return c.Next()
	})

	handler := func(c *fiber.Ctx) error {
		return c.SendString("ok")
	}

	app.Get("/resource", BlockImpersonationWrites(), handler)
	app.Post("/resource", BlockImpersonationWrites(), handler)
	app.Put("/resource", BlockImpersonationWrites(), handler)
	app.Patch("/resource", BlockImpersonationWrites(), handler)
	app.Delete("/resource", BlockImpersonationWrites(), handler)

	return app
}

func TestBlockImpersonationWrites_ReadOnlyBlocksMutations(t *testing.T) {
	app := appWithImpersonation(true)

	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		resp, err := app.Test(httptest.NewRequest(method, "/resource", nil))
		require.NoError(t, err)
		assert.Equalf(t, fiber.StatusForbidden, resp.StatusCode, "%s should be blocked", method)
		var body fiberctx.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, "permission.read_only", body.Code)
	}
}

func TestBlockImpersonationWrites_ReadOnlyAllowsReads(t *testing.T) {
	app := appWithImpersonation(true)

	resp, err := app.Test(httptest.NewRequest("GET", "/resource", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestBlockImpersonationWrites_NormalRequestAllowsWrites(t *testing.T) {
	app := appWithImpersonation(false)

	resp, err := app.Test(httptest.NewRequest("POST", "/resource", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

const testAuthSecret = "test-impersonation-secret"

// signImpersonationToken mints an HS256 access token. When readOnly is true it
// carries the impersonation claims that setAuthContext surfaces into locals,
// which Auth() uses to enforce the read-only contract.
func signImpersonationToken(t *testing.T, readOnly bool) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub":  "target",
		"type": "access",
		"exp":  time.Now().Add(time.Hour).Unix(),
	}

	if readOnly {
		claims["impersonation_sid"] = "sess1"
		claims["read_only"] = true
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testAuthSecret))
	require.NoError(t, err)

	return signed
}

// appWithAuth wires the real Auth middleware as the route guard. A valid
// impersonation JWT short-circuits before the PAT fallback ever touches the DB,
// so a nil *gorm.DB is sufficient for these JWT-success cases.
func appWithAuth() *fiber.App {
	app := fiber.New()
	app.Use(Locale(i18n.LocaleEnglish))

	handler := func(c *fiber.Ctx) error {
		return c.SendString("ok")
	}

	app.Get("/resource", Auth(testAuthSecret, nil), handler)
	app.Post("/resource", Auth(testAuthSecret, nil), handler)

	return app
}

// TestAuth_ReadOnlyImpersonationBlocksWrites locks in the REAL enforcement
// chokepoint: Auth() must reject mutating requests carrying a read-only
// impersonation token, allow reads with the same token, and leave normal tokens
// untouched.
func TestAuth_ReadOnlyImpersonationBlocksWrites(t *testing.T) {
	app := appWithAuth()

	readOnlyToken := signImpersonationToken(t, true)
	normalToken := signImpersonationToken(t, false)

	t.Run("POST with read-only token is blocked", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/resource", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)
		req.Header.Set("Accept-Language", "ja")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
		defer resp.Body.Close()
		var body fiberctx.ErrorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, "permission.read_only", body.Code)
		assert.Equal(t, "このアカウントは読み取り専用モードで表示されています。変更するには閲覧モードを終了してください。", body.Message)
	})

	t.Run("GET with read-only token passes", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/resource", nil)
		req.Header.Set("Authorization", "Bearer "+readOnlyToken)

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("POST with normal token passes", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/resource", nil)
		req.Header.Set("Authorization", "Bearer "+normalToken)

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}
