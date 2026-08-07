package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hookDefaultsData struct {
	HookAfterUpdatingRepository string `json:"hook_after_updating_repository"`
}

type hookDefaultsResponseBody struct {
	Data hookDefaultsData `json:"data"`
}

func TestGetHookDefaults(t *testing.T) {
	app := fiber.New()
	handler := NewSiteHandler(nil)
	app.Get("/sites/hook-defaults", handler.GetHookDefaults)

	t.Run("returns the build script for a known type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sites/hook-defaults?type=laravel&zero_downtime=false", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var body hookDefaultsResponseBody
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Contains(t, body.Data.HookAfterUpdatingRepository, "composer install")
	})

	t.Run("zero_downtime defaults to false when omitted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sites/hook-defaults?type=laravel", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("rejects a missing type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sites/hook-defaults", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("rejects an unknown type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/sites/hook-defaults?type=not-a-real-type", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}
