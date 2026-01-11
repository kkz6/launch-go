package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
)

type testResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
	Errors  interface{}     `json:"errors,omitempty"`
}

func setupTestApp(t *testing.T) (*fiber.App, *Handler, *Service, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&User{}, &Team{}, &TeamMember{}, &TeamInvitation{}, &PersonalAccessToken{}, &PasswordResetToken{})
	require.NoError(t, err)

	repo := NewRepository(db)
	cfg := &config.Config{
		App: config.AppConfig{
			Name: "TestApp",
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret-key-for-jwt-testing",
			Expiration: 24,
		},
	}
	logger := zerolog.Nop()
	service := NewService(repo, cfg, &logger)
	handler := NewHandler(service)

	app := fiber.New()

	return app, handler, service, db
}

func createTestUserForHandler(t *testing.T, db *gorm.DB, email, password string) *User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &User{
		Name:     "Test User",
		Email:    email,
		Password: string(hashedPassword),
		Timezone: "UTC",
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	return user
}

func makeRequest(app *fiber.App, method, path string, body interface{}, userID string) (*http.Response, []byte) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	if userID != "" {
		req.Header.Set("X-Test-User-ID", userID)
	}

	resp, _ := app.Test(req, -1)
	respBody, _ := io.ReadAll(resp.Body)

	return resp, respBody
}

func withAuthMiddleware(app *fiber.App, handler func(c *fiber.Ctx) error) {
	app.Use(func(c *fiber.Ctx) error {
		userID := c.Get("X-Test-User-ID")
		if userID != "" {
			c.Locals("userID", userID)
		}
		return c.Next()
	})
}

// Authentication Handler Tests

func TestHandler_Register(t *testing.T) {
	app, handler, _, _ := setupTestApp(t)
	app.Post("/register", handler.Register)

	t.Run("successful registration", func(t *testing.T) {
		body := map[string]interface{}{
			"name":                  "John Doe",
			"email":                 "john@example.com",
			"password":              "password123",
			"password_confirmation": "password123",
		}

		resp, respBody := makeRequest(app, "POST", "/register", body, "")
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response testResponse
		err := json.Unmarshal(respBody, &response)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "Registration successful", response.Message)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/register", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error - missing fields", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "John Doe",
		}

		resp, _ := makeRequest(app, "POST", "/register", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("duplicate email", func(t *testing.T) {
		// First registration
		body := map[string]interface{}{
			"name":                  "User One",
			"email":                 "duplicate@example.com",
			"password":              "password123",
			"password_confirmation": "password123",
		}
		makeRequest(app, "POST", "/register", body, "")

		// Duplicate registration
		body["name"] = "User Two"
		resp, _ := makeRequest(app, "POST", "/register", body, "")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_Login(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	app.Post("/login", handler.Login)

	createTestUserForHandler(t, db, "login@example.com", "password123")

	t.Run("successful login", func(t *testing.T) {
		body := map[string]interface{}{
			"email":    "login@example.com",
			"password": "password123",
		}

		resp, respBody := makeRequest(app, "POST", "/login", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
		assert.Equal(t, "Login successful", response.Message)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		body := map[string]interface{}{
			"email":    "login@example.com",
			"password": "wrongpassword",
		}

		resp, _ := makeRequest(app, "POST", "/login", body, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/login", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "login@example.com",
		}

		resp, _ := makeRequest(app, "POST", "/login", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_Logout(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.Logout)
	app.Post("/logout", handler.Logout)

	user := createTestUserForHandler(t, db, "logout@example.com", "password")

	t.Run("successful logout", func(t *testing.T) {
		resp, respBody := makeRequest(app, "POST", "/logout", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})
}

func TestHandler_RefreshToken(t *testing.T) {
	app, handler, service, db := setupTestApp(t)
	app.Post("/refresh", handler.RefreshToken)

	createTestUserForHandler(t, db, "refresh@example.com", "password")

	t.Run("successful refresh", func(t *testing.T) {
		// Login first
		loginResp, err := service.Login(nil, &LoginRequest{
			Email:    "refresh@example.com",
			Password: "password",
		})
		require.NoError(t, err)

		body := map[string]interface{}{
			"refresh_token": loginResp.RefreshToken,
		}

		resp, respBody := makeRequest(app, "POST", "/refresh", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})

	t.Run("invalid token", func(t *testing.T) {
		body := map[string]interface{}{
			"refresh_token": "invalid-token",
		}

		resp, _ := makeRequest(app, "POST", "/refresh", body, "")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/refresh", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/refresh", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

// User Management Handler Tests

func TestHandler_User(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.User)
	app.Get("/user", handler.User)

	user := createTestUserForHandler(t, db, "user@example.com", "password")

	t.Run("returns current user", func(t *testing.T) {
		resp, respBody := makeRequest(app, "GET", "/user", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/user", nil, "non-existent-id")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_UpdateProfile(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.UpdateProfile)
	app.Put("/profile", handler.UpdateProfile)

	user := createTestUserForHandler(t, db, "profile@example.com", "password")

	t.Run("successful update", func(t *testing.T) {
		body := map[string]interface{}{
			"name":  "Updated Name",
			"email": "profile@example.com",
		}

		resp, respBody := makeRequest(app, "PUT", "/profile", body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/profile", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "",
		}

		resp, _ := makeRequest(app, "PUT", "/profile", body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_ChangePassword(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.ChangePassword)
	app.Put("/password", handler.ChangePassword)

	user := createTestUserForHandler(t, db, "changepass@example.com", "oldpassword")

	t.Run("successful change", func(t *testing.T) {
		body := map[string]interface{}{
			"current_password":      "oldpassword",
			"password":              "newpassword",
			"password_confirmation": "newpassword",
		}

		resp, _ := makeRequest(app, "PUT", "/password", body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/password", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{
			"current_password": "old",
		}

		resp, _ := makeRequest(app, "PUT", "/password", body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_DeleteAccount(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.DeleteAccount)
	app.Delete("/account", handler.DeleteAccount)

	user := createTestUserForHandler(t, db, "delete@example.com", "password")

	t.Run("successful delete", func(t *testing.T) {
		resp, _ := makeRequest(app, "DELETE", "/account", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Email Verification Handler Tests

func TestHandler_VerifyEmail(t *testing.T) {
	app, handler, service, db := setupTestApp(t)
	app.Get("/verify/:id/:hash", handler.VerifyEmail)

	user := createTestUserForHandler(t, db, "verify@example.com", "password")
	hash := service.generateEmailHash("verify@example.com")

	t.Run("successful verification", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/verify/"+user.ID+"/"+hash, nil, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid hash", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/verify/"+user.ID+"/invalid-hash", nil, "")
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_ResendVerificationEmail(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.ResendVerificationEmail)
	app.Post("/resend", handler.ResendVerificationEmail)

	user := createTestUserForHandler(t, db, "resend@example.com", "password")

	t.Run("successful resend", func(t *testing.T) {
		resp, _ := makeRequest(app, "POST", "/resend", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("already verified", func(t *testing.T) {
		now := time.Now()
		user.EmailVerifiedAt = &now
		db.Save(user)

		resp, _ := makeRequest(app, "POST", "/resend", nil, user.ID)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// Password Reset Handler Tests

func TestHandler_ForgotPassword(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	app.Post("/forgot", handler.ForgotPassword)

	createTestUserForHandler(t, db, "forgot@example.com", "password")

	t.Run("successful request", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "forgot@example.com",
		}

		resp, _ := makeRequest(app, "POST", "/forgot", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("non-existent email also returns success", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "nonexistent@example.com",
		}

		resp, _ := makeRequest(app, "POST", "/forgot", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/forgot", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/forgot", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_ResetPassword(t *testing.T) {
	app, handler, service, db := setupTestApp(t)
	app.Post("/reset", handler.ResetPassword)

	createTestUserForHandler(t, db, "reset@example.com", "oldpassword")

	t.Run("successful reset", func(t *testing.T) {
		// Create a token
		token := "test-token-123"
		hashedToken := service.hashToken(token)
		db.Create(&PasswordResetToken{
			Email:     "reset@example.com",
			Token:     hashedToken,
			CreatedAt: time.Now(),
		})

		body := map[string]interface{}{
			"email":                 "reset@example.com",
			"token":                 token,
			"password":              "newpassword",
			"password_confirmation": "newpassword",
		}

		resp, _ := makeRequest(app, "POST", "/reset", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/reset", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "test@test.com",
		}

		resp, _ := makeRequest(app, "POST", "/reset", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

// Two-Factor Authentication Handler Tests

func TestHandler_EnableTwoFactor(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.EnableTwoFactor)
	app.Post("/2fa/enable", handler.EnableTwoFactor)

	user := createTestUserForHandler(t, db, "enable2fa@example.com", "password")

	t.Run("successful enable", func(t *testing.T) {
		resp, respBody := makeRequest(app, "POST", "/2fa/enable", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})
}

func TestHandler_ConfirmTwoFactor(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.ConfirmTwoFactor)
	app.Post("/2fa/confirm", handler.ConfirmTwoFactor)

	user := createTestUserForHandler(t, db, "confirm2fa@example.com", "password")

	t.Run("invalid code", func(t *testing.T) {
		// Enable 2FA first
		secret := "JBSWY3DPEHPK3PXP"
		user.TwoFactorSecret = &secret
		db.Save(user)

		body := map[string]interface{}{
			"code": "000000",
		}

		resp, _ := makeRequest(app, "POST", "/2fa/confirm", body, user.ID)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/2fa/confirm", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/2fa/confirm", body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_DisableTwoFactor(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.DisableTwoFactor)
	app.Delete("/2fa/disable", handler.DisableTwoFactor)

	user := createTestUserForHandler(t, db, "disable2fa@example.com", "password")

	t.Run("successful disable", func(t *testing.T) {
		// Enable 2FA first
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		body := map[string]interface{}{
			"password": "password",
		}

		resp, _ := makeRequest(app, "DELETE", "/2fa/disable", body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/2fa/disable", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_TwoFactorChallenge(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.TwoFactorChallenge)
	app.Post("/2fa/challenge", handler.TwoFactorChallenge)

	user := createTestUserForHandler(t, db, "challenge@example.com", "password")

	t.Run("2FA not enabled", func(t *testing.T) {
		body := map[string]interface{}{
			"code": "123456",
		}

		resp, _ := makeRequest(app, "POST", "/2fa/challenge", body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid code", func(t *testing.T) {
		// Enable 2FA
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		body := map[string]interface{}{
			"code": "000000",
		}

		resp, _ := makeRequest(app, "POST", "/2fa/challenge", body, user.ID)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("recovery code", func(t *testing.T) {
		codes := "RECOVERY-CODE-1,RECOVERY-CODE-2"
		user.TwoFactorRecoveryCodes = &codes
		db.Save(user)

		body := map[string]interface{}{
			"recovery_code": "RECOVERY-CODE-1",
		}

		resp, _ := makeRequest(app, "POST", "/2fa/challenge", body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/2fa/challenge", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_GetRecoveryCodes(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.GetRecoveryCodes)
	app.Get("/2fa/recovery", handler.GetRecoveryCodes)

	user := createTestUserForHandler(t, db, "recovery@example.com", "password")
	codes := "CODE1,CODE2,CODE3"
	user.TwoFactorRecoveryCodes = &codes
	db.Save(user)

	t.Run("returns codes", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/2fa/recovery", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_RegenerateRecoveryCodes(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.RegenerateRecoveryCodes)
	app.Post("/2fa/recovery", handler.RegenerateRecoveryCodes)

	user := createTestUserForHandler(t, db, "regen@example.com", "password")

	t.Run("2FA not enabled", func(t *testing.T) {
		resp, _ := makeRequest(app, "POST", "/2fa/recovery", nil, user.ID)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("successful regeneration", func(t *testing.T) {
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		resp, _ := makeRequest(app, "POST", "/2fa/recovery", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Team Management Handler Tests

func TestHandler_CreateTeam(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.CreateTeam)
	app.Post("/teams", handler.CreateTeam)

	user := createTestUserForHandler(t, db, "createteam@example.com", "password")

	t.Run("successful creation", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "New Team",
		}

		resp, respBody := makeRequest(app, "POST", "/teams", body, user.ID)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/teams", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/teams", body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_GetTeam(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.GetTeam)
	app.Get("/teams/:teamId", handler.GetTeam)

	user := createTestUserForHandler(t, db, "getteam@example.com", "password")
	team := &Team{Name: "Test Team", OwnerID: user.ID}
	db.Create(team)

	t.Run("returns team", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/teams/"+team.ID, nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("team not found", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/teams/non-existent", nil, user.ID)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_UpdateTeam(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.UpdateTeam)
	app.Put("/teams/:teamId", handler.UpdateTeam)

	user := createTestUserForHandler(t, db, "updateteam@example.com", "password")
	team := &Team{Name: "Original Name", OwnerID: user.ID}
	db.Create(team)

	t.Run("successful update", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Updated Name",
		}

		resp, _ := makeRequest(app, "PUT", "/teams/"+team.ID, body, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/teams/"+team.ID, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "PUT", "/teams/"+team.ID, body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_DeleteTeam(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.DeleteTeam)
	app.Delete("/teams/:teamId", handler.DeleteTeam)

	user := createTestUserForHandler(t, db, "deleteteam@example.com", "password")
	team := &Team{Name: "To Delete", OwnerID: user.ID, PersonalTeam: false}
	db.Create(team)

	t.Run("successful delete", func(t *testing.T) {
		resp, _ := makeRequest(app, "DELETE", "/teams/"+team.ID, nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_GetUserTeams(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.GetUserTeams)
	app.Get("/user/teams", handler.GetUserTeams)

	user := createTestUserForHandler(t, db, "userteams@example.com", "password")
	db.Create(&Team{Name: "Team 1", OwnerID: user.ID})
	db.Create(&Team{Name: "Team 2", OwnerID: user.ID})

	t.Run("returns teams", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/user/teams", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_SwitchTeam(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.SwitchTeam)
	app.Post("/teams/:teamId/switch", handler.SwitchTeam)

	user := createTestUserForHandler(t, db, "switchteam@example.com", "password")
	team := &Team{Name: "Switch To", OwnerID: user.ID}
	db.Create(team)
	db.Create(&TeamMember{TeamID: team.ID, UserID: user.ID, Role: "owner"})

	t.Run("successful switch", func(t *testing.T) {
		resp, _ := makeRequest(app, "POST", "/teams/"+team.ID+"/switch", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Team Member Management Handler Tests

func TestHandler_InviteTeamMember(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.InviteTeamMember)
	app.Post("/teams/:teamId/members", handler.InviteTeamMember)

	user := createTestUserForHandler(t, db, "inviteowner@example.com", "password")
	team := &Team{Name: "Invite Team", OwnerID: user.ID}
	db.Create(team)

	t.Run("successful invite", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "newinvite@example.com",
			"role":  "member",
		}

		resp, _ := makeRequest(app, "POST", "/teams/"+team.ID+"/members", body, user.ID)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/teams/"+team.ID+"/members", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", user.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/teams/"+team.ID+"/members", body, user.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_AcceptTeamInvitation(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.AcceptTeamInvitation)
	app.Post("/invitations/:invitationId/accept", handler.AcceptTeamInvitation)

	owner := createTestUserForHandler(t, db, "acceptowner@example.com", "password")
	invitee := createTestUserForHandler(t, db, "invitee@example.com", "password")
	team := &Team{Name: "Accept Team", OwnerID: owner.ID}
	db.Create(team)

	invitation := &TeamInvitation{
		TeamID: team.ID,
		Email:  "invitee@example.com",
		Role:   "member",
	}
	db.Create(invitation)

	t.Run("successful accept", func(t *testing.T) {
		resp, _ := makeRequest(app, "POST", "/invitations/"+invitation.ID+"/accept", nil, invitee.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_CancelTeamInvitation(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.CancelTeamInvitation)
	app.Delete("/teams/:teamId/invitations/:invitationId", handler.CancelTeamInvitation)

	user := createTestUserForHandler(t, db, "cancelowner@example.com", "password")
	team := &Team{Name: "Cancel Team", OwnerID: user.ID}
	db.Create(team)

	invitation := &TeamInvitation{
		TeamID: team.ID,
		Email:  "cancel@example.com",
		Role:   "member",
	}
	db.Create(invitation)

	t.Run("successful cancel", func(t *testing.T) {
		resp, _ := makeRequest(app, "DELETE", "/teams/"+team.ID+"/invitations/"+invitation.ID, nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_UpdateTeamMemberRole(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.UpdateTeamMemberRole)
	app.Put("/teams/:teamId/members/:memberId", handler.UpdateTeamMemberRole)

	owner := createTestUserForHandler(t, db, "roleowner@example.com", "password")
	member := createTestUserForHandler(t, db, "rolemember@example.com", "password")
	team := &Team{Name: "Role Team", OwnerID: owner.ID}
	db.Create(team)
	db.Create(&TeamMember{TeamID: team.ID, UserID: member.ID, Role: "member"})

	t.Run("successful update", func(t *testing.T) {
		body := map[string]interface{}{
			"role": "admin",
		}

		resp, _ := makeRequest(app, "PUT", "/teams/"+team.ID+"/members/"+member.ID, body, owner.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/teams/"+team.ID+"/members/"+member.ID, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", owner.ID)

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "PUT", "/teams/"+team.ID+"/members/"+member.ID, body, owner.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_RemoveTeamMember(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.RemoveTeamMember)
	app.Delete("/teams/:teamId/members/:memberId", handler.RemoveTeamMember)

	owner := createTestUserForHandler(t, db, "removeowner@example.com", "password")
	member := createTestUserForHandler(t, db, "removemember@example.com", "password")
	team := &Team{Name: "Remove Team", OwnerID: owner.ID}
	db.Create(team)
	db.Create(&TeamMember{TeamID: team.ID, UserID: member.ID, Role: "member"})

	t.Run("successful remove", func(t *testing.T) {
		resp, _ := makeRequest(app, "DELETE", "/teams/"+team.ID+"/members/"+member.ID, nil, owner.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_GetTeamMembers(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.GetTeamMembers)
	app.Get("/teams/:teamId/members", handler.GetTeamMembers)

	user := createTestUserForHandler(t, db, "getmembers@example.com", "password")
	team := &Team{Name: "Members Team", OwnerID: user.ID}
	db.Create(team)
	db.Create(&TeamMember{TeamID: team.ID, UserID: user.ID, Role: "owner"})

	t.Run("returns members", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/teams/"+team.ID+"/members", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_GetTeamInvitations(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	withAuthMiddleware(app, handler.GetTeamInvitations)
	app.Get("/teams/:teamId/invitations", handler.GetTeamInvitations)

	user := createTestUserForHandler(t, db, "getinvitations@example.com", "password")
	team := &Team{Name: "Invitations Team", OwnerID: user.ID}
	db.Create(team)
	db.Create(&TeamInvitation{TeamID: team.ID, Email: "invite@example.com", Role: "member"})

	t.Run("returns invitations", func(t *testing.T) {
		resp, _ := makeRequest(app, "GET", "/teams/"+team.ID+"/invitations", nil, user.ID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// User Status Handler Tests

func TestHandler_CheckUserStatus(t *testing.T) {
	app, handler, _, db := setupTestApp(t)
	app.Post("/check-status", handler.CheckUserStatus)

	createTestUserForHandler(t, db, "status@example.com", "password")

	t.Run("existing user", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "status@example.com",
		}

		resp, respBody := makeRequest(app, "POST", "/check-status", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response testResponse
		json.Unmarshal(respBody, &response)
		assert.True(t, response.Success)
	})

	t.Run("non-existent user", func(t *testing.T) {
		body := map[string]interface{}{
			"email": "nobody@example.com",
		}

		resp, _ := makeRequest(app, "POST", "/check-status", body, "")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/check-status", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation error", func(t *testing.T) {
		body := map[string]interface{}{}

		resp, _ := makeRequest(app, "POST", "/check-status", body, "")
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

// NewHandler Test

func TestNewHandler(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := NewRepository(db)
	cfg := &config.Config{}
	logger := zerolog.Nop()
	service := NewService(repo, cfg, &logger)

	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.service)
}
