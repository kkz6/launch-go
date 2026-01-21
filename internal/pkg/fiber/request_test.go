package fiber

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type testRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type testQueryParams struct {
	Page    int    `query:"page"`
	PerPage int    `query:"per_page"`
	Search  string `query:"search"`
}

func TestParseAndValidate(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		contentType    string
		expectError    bool
		expectedStatus int
	}{
		{
			name:           "valid request",
			body:           `{"name":"Test","email":"test@example.com"}`,
			contentType:    "application/json",
			expectError:    false,
			expectedStatus: 0, // No response sent
		},
		{
			name:           "invalid json",
			body:           `{"name":}`,
			contentType:    "application/json",
			expectError:    true,
			expectedStatus: fiber.StatusBadRequest,
		},
		{
			name:           "validation failure - missing required",
			body:           `{"name":"Test"}`,
			contentType:    "application/json",
			expectError:    true,
			expectedStatus: fiber.StatusUnprocessableEntity,
		},
		{
			name:           "validation failure - invalid email",
			body:           `{"name":"Test","email":"invalid"}`,
			contentType:    "application/json",
			expectError:    true,
			expectedStatus: fiber.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Post("/test", func(c *fiber.Ctx) error {
				var req testRequest
				if err := ParseAndValidate(c, &req); err != nil {
					return err
				}
				return c.SendString("OK")
			})

			req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}

			if tt.expectError {
				if resp.StatusCode != tt.expectedStatus {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
				}
			} else {
				if resp.StatusCode != fiber.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
				}
			}
		})
	}
}

func TestMustParseAndValidate(t *testing.T) {
	app := fiber.New()
	app.Post("/test", func(c *fiber.Ctx) error {
		req, err := MustParseAndValidate[testRequest](c)
		if err != nil {
			return err
		}
		return c.JSON(req)
	})

	// Valid request
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(`{"name":"Test","email":"test@example.com"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}
}

func TestParseQuery(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		params, err := ParseQuery[testQueryParams](c)
		if err != nil {
			return err
		}
		return c.JSON(params)
	})

	req := httptest.NewRequest("GET", "/test?page=1&per_page=10&search=hello", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}
}
