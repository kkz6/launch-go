package fiber

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetTeamID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*fiber.Ctx)
		want    string
		wantErr bool
	}{
		{
			name: "valid team ID",
			setup: func(c *fiber.Ctx) {
				c.Locals("teamID", "team123")
			},
			want:    "team123",
			wantErr: false,
		},
		{
			name:    "missing team ID",
			setup:   func(c *fiber.Ctx) {},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty team ID",
			setup: func(c *fiber.Ctx) {
				c.Locals("teamID", "")
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "wrong type",
			setup: func(c *fiber.Ctx) {
				c.Locals("teamID", 123)
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				tt.setup(c)
				got, err := GetTeamID(c)
				if (err != nil) != tt.wantErr {
					t.Errorf("GetTeamID() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if got != tt.want {
					t.Errorf("GetTeamID() = %v, want %v", got, tt.want)
				}
				return nil
			})

			req := httptest.NewRequest("GET", "/test", nil)
			_, _ = app.Test(req)
		})
	}
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*fiber.Ctx)
		want    string
		wantErr bool
	}{
		{
			name: "valid user ID",
			setup: func(c *fiber.Ctx) {
				c.Locals("userID", "user456")
			},
			want:    "user456",
			wantErr: false,
		},
		{
			name:    "missing user ID",
			setup:   func(c *fiber.Ctx) {},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty user ID",
			setup: func(c *fiber.Ctx) {
				c.Locals("userID", "")
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				tt.setup(c)
				got, err := GetUserID(c)
				if (err != nil) != tt.wantErr {
					t.Errorf("GetUserID() error = %v, wantErr %v", err, tt.wantErr)
					return nil
				}
				if got != tt.want {
					t.Errorf("GetUserID() = %v, want %v", got, tt.want)
				}
				return nil
			})

			req := httptest.NewRequest("GET", "/test", nil)
			_, _ = app.Test(req)
		})
	}
}

func TestMustGetTeamID(t *testing.T) {
	// Test success case
	t.Run("success", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("teamID", "team123")
			got, err := MustGetTeamID(c)
			if err != nil {
				return err
			}
			if got != "team123" {
				t.Errorf("MustGetTeamID() = %v, want team123", got)
			}
			return c.SendStatus(200)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Test error case - MustGetTeamID sends 401 response directly
	t.Run("missing teamID returns 401", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			_, err := MustGetTeamID(c)
			return err
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != 401 {
			t.Errorf("expected 401 for missing teamID, got %d", resp.StatusCode)
		}
	})
}

func TestGetTeamAndUserID(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("teamID", "team123")
		c.Locals("userID", "user456")

		teamID, userID, err := GetTeamAndUserID(c)
		if err != nil {
			t.Errorf("GetTeamAndUserID() unexpected error: %v", err)
			return nil
		}
		if teamID != "team123" {
			t.Errorf("teamID = %v, want team123", teamID)
		}
		if userID != "user456" {
			t.Errorf("userID = %v, want user456", userID)
		}
		return nil
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req)
}

type testUser struct {
	ID   string
	Name string
}

func TestGetUser(t *testing.T) {
	app := fiber.New()

	app.Get("/test", func(c *fiber.Ctx) error {
		user := &testUser{ID: "123", Name: "Test"}
		c.Locals("user", user)

		got, err := GetUser[testUser](c)
		if err != nil {
			t.Errorf("GetUser() unexpected error: %v", err)
			return nil
		}
		if got.ID != "123" || got.Name != "Test" {
			t.Errorf("GetUser() = %+v, want {ID:123, Name:Test}", got)
		}
		return nil
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req)
}
