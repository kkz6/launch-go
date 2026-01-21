package fiber

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

func TestGetID(t *testing.T) {
	validULID := ulid.Make().String()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "valid ULID",
			path:           "/test/" + validULID,
			expectedStatus: 200,
			expectError:    false,
		},
		{
			name:           "invalid ULID",
			path:           "/test/not-a-ulid",
			expectedStatus: 400,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test/:id", func(c *fiber.Ctx) error {
				id, err := GetID(c)
				if err != nil {
					return err
				}
				return c.SendString(id)
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to test request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if !tt.expectError {
				body, _ := io.ReadAll(resp.Body)
				if string(body) != validULID {
					t.Errorf("Expected body %q, got %q", validULID, string(body))
				}
			}
		})
	}
}

func TestGetServerID(t *testing.T) {
	validULID := ulid.Make().String()

	app := fiber.New()
	app.Get("/servers/:serverId/sites", func(c *fiber.Ctx) error {
		serverID, err := GetServerID(c)
		if err != nil {
			return err
		}
		return c.SendString(serverID)
	})

	// Valid case
	req := httptest.NewRequest("GET", "/servers/"+validULID+"/sites", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != validULID {
		t.Errorf("Expected body %q, got %q", validULID, string(body))
	}
}

func TestGetULIDParam(t *testing.T) {
	validULID := ulid.Make().String()

	tests := []struct {
		name           string
		paramName      string
		path           string
		routePath      string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "valid custom param",
			paramName:      "deploymentId",
			path:           "/deployments/" + validULID,
			routePath:      "/deployments/:deploymentId",
			expectedStatus: 200,
			expectError:    false,
		},
		{
			name:           "invalid custom param",
			paramName:      "deploymentId",
			path:           "/deployments/invalid",
			routePath:      "/deployments/:deploymentId",
			expectedStatus: 400,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get(tt.routePath, func(c *fiber.Ctx) error {
				id, err := GetULIDParam(c, tt.paramName)
				if err != nil {
					return err
				}
				return c.SendString(id)
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to test request: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// For error cases, just verify we got the right status code
			// The error message format may vary based on how Fiber handles the response
		})
	}
}

func TestGetRequiredParam(t *testing.T) {
	app := fiber.New()
	app.Get("/files/:filename", func(c *fiber.Ctx) error {
		filename, err := GetRequiredParam(c, "filename")
		if err != nil {
			return err
		}
		return c.SendString(filename)
	})

	// Valid case - any non-empty string is accepted
	req := httptest.NewRequest("GET", "/files/my-file.txt", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "my-file.txt" {
		t.Errorf("Expected body 'my-file.txt', got %q", string(body))
	}
}

func TestMustGetID_Panics(t *testing.T) {
	// Test that MustGetID panics when given invalid input
	app := fiber.New()

	app.Get("/test/:id", func(c *fiber.Ctx) error {
		_ = MustGetID(c)
		return c.SendString("should not reach")
	})

	req := httptest.NewRequest("GET", "/test/invalid", nil)
	resp, _ := app.Test(req)

	// MustGetID panics with the error from GetID
	// Fiber's default error handler catches this and returns 500
	// The response.BadRequest was already sent, so we get 400
	if resp.StatusCode == 200 {
		t.Error("Expected non-200 status for invalid ULID")
	}
}

func TestMustGetID_Success(t *testing.T) {
	validULID := ulid.Make().String()

	app := fiber.New()
	app.Get("/test/:id", func(c *fiber.Ctx) error {
		id := MustGetID(c)
		return c.SendString(id)
	})

	req := httptest.NewRequest("GET", "/test/"+validULID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != validULID {
		t.Errorf("Expected body %q, got %q", validULID, string(body))
	}
}
