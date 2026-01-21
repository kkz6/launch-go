package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestValidateULIDParams(t *testing.T) {
	tests := []struct {
		name           string
		params         []string
		path           string
		routePath      string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid single ULID",
			params:         []string{"id"},
			path:           "/test/" + ulid.Make().String(),
			routePath:      "/test/:id",
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		{
			name:           "valid multiple ULIDs",
			params:         []string{"serverId", "id"},
			path:           "/servers/" + ulid.Make().String() + "/sites/" + ulid.Make().String(),
			routePath:      "/servers/:serverId/sites/:id",
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		// Note: Fiber won't match /test/ to /test/:id route - it requires a non-empty param
		// The middleware handles cases where params exist but have invalid format
		{
			name:           "invalid ULID format",
			params:         []string{"id"},
			path:           "/test/not-a-ulid",
			routePath:      "/test/:id",
			expectedStatus: 400,
			expectedBody:   `"message":"Invalid parameter format: id"`,
		},
		{
			name:           "one valid one invalid",
			params:         []string{"serverId", "id"},
			path:           "/servers/" + ulid.Make().String() + "/sites/invalid",
			routePath:      "/servers/:serverId/sites/:id",
			expectedStatus: 400,
			expectedBody:   `"message":"Invalid parameter format: id"`,
		},
		{
			name:           "UUID format not accepted",
			params:         []string{"id"},
			path:           "/test/550e8400-e29b-41d4-a716-446655440000",
			routePath:      "/test/:id",
			expectedStatus: 400,
			expectedBody:   `"message":"Invalid parameter format: id"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get(tt.routePath, ValidateULIDParams(tt.params...), func(c *fiber.Ctx) error {
				return c.SendString("ok")
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to test request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if tt.expectedBody != "" && !contains(string(body), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, string(body))
			}
		})
	}
}

func TestRequireParams(t *testing.T) {
	tests := []struct {
		name           string
		params         []string
		path           string
		routePath      string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid single param",
			params:         []string{"filename"},
			path:           "/files/test.txt",
			routePath:      "/files/:filename",
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		{
			name:           "valid multiple params",
			params:         []string{"folder", "filename"},
			path:           "/files/docs/readme.md",
			routePath:      "/files/:folder/:filename",
			expectedStatus: 200,
			expectedBody:   "ok",
		},
		// Note: Fiber won't match /files/ to /files/:filename route - it requires a non-empty param
		// The middleware handles cases where params exist but are defined incorrectly in routes
		{
			name:           "accepts any non-empty string",
			params:         []string{"slug"},
			path:           "/posts/my-awesome-post-123",
			routePath:      "/posts/:slug",
			expectedStatus: 200,
			expectedBody:   "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get(tt.routePath, RequireParams(tt.params...), func(c *fiber.Ctx) error {
				return c.SendString("ok")
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to test request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if tt.expectedBody != "" && !contains(string(body), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, string(body))
			}
		})
	}
}

func TestValidateULIDParams_MiddlewareChain(t *testing.T) {
	validULID := ulid.Make().String()
	callOrder := []string{}

	app := fiber.New()
	app.Get("/test/:id",
		func(c *fiber.Ctx) error {
			callOrder = append(callOrder, "before")
			return c.Next()
		},
		ValidateULIDParams("id"),
		func(c *fiber.Ctx) error {
			callOrder = append(callOrder, "after")
			return c.SendString("ok")
		},
	)

	// Test valid request - all middleware should run
	callOrder = []string{}
	req := httptest.NewRequest("GET", "/test/"+validULID, nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if len(callOrder) != 2 || callOrder[0] != "before" || callOrder[1] != "after" {
		t.Errorf("Expected middleware chain to run in order, got %v", callOrder)
	}

	// Test invalid request - should stop at validation
	callOrder = []string{}
	req = httptest.NewRequest("GET", "/test/invalid", nil)
	_, err = app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if len(callOrder) != 1 || callOrder[0] != "before" {
		t.Errorf("Expected middleware chain to stop at validation, got %v", callOrder)
	}
}
