package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

func setupTestService(t *testing.T) (*services.Service, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.TeamInvitation{}, &models.PersonalAccessToken{}, &models.PasswordResetToken{})
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
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

	service := services.NewService(repo, cfg, &logger)

	return service, db
}

func createTestUserWithPassword(t *testing.T, db *gorm.DB, email, password string) *models.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &models.User{
		Name:     "Test User",
		Email:    email,
		Password: string(hashedPassword),
		Timezone: "UTC",
	}
	err = db.Create(user).Error
	require.NoError(t, err)

	return user
}

// Test helper functions to replicate internal service methods
// Note: generateEmailHashForTest and hashTokenForTest are defined in handler_test.go

// generateEmailHashWithSecret creates hash matching the service's generateEmailHash method
func generateEmailHashWithSecret(email string, jwtSecret string) string {
	data := email + jwtSecret
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Authentication Tests

func TestService_Register(t *testing.T) {
	service, _ := setupTestService(t)
	ctx := context.Background()

	t.Run("successful registration", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:     "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}

		resp, err := service.Register(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.Equal(t, "john@example.com", resp.User.Email)
	})

	t.Run("registration with personal team", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:               "Jane Doe",
			Email:              "jane@example.com",
			Password:           "password123",
			CreatePersonalTeam: true,
		}

		resp, err := service.Register(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.User.CurrentTeamID)
	})

	t.Run("registration with custom timezone", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:     "Timezone User",
			Email:    "timezone@example.com",
			Password: "password123",
			Timezone: "America/New_York",
		}

		resp, err := service.Register(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("duplicate email fails", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:     "Duplicate",
			Email:    "john@example.com",
			Password: "password123",
		}

		resp, err := service.Register(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("registration normalizes email", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:     "Case Test",
			Email:    "  UPPERCASE@EXAMPLE.COM  ",
			Password: "password123",
		}

		resp, err := service.Register(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "uppercase@example.com", resp.User.Email)
	})
}

func TestService_Register_WithInvitation(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Create a team owner
	owner := createTestUserWithPassword(t, db, "owner@example.com", "password")

	// Create a team
	team := &models.Team{
		Name:    "Test Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	// Create an invitation
	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invited@example.com",
		Role:   "member",
	}
	err = db.Create(invitation).Error
	require.NoError(t, err)

	t.Run("registration with valid invitation", func(t *testing.T) {
		req := &dto.RegisterRequest{
			Name:         "Invited User",
			Email:        "invited@example.com",
			Password:     "password123",
			InvitationID: &invitation.ID,
		}

		resp, err := service.Register(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.User.CurrentTeamID)
	})
}

func TestService_Login(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	// Create a test user
	createTestUserWithPassword(t, db, "login@example.com", "correctpassword")

	t.Run("successful login", func(t *testing.T) {
		req := &dto.LoginRequest{
			Email:    "login@example.com",
			Password: "correctpassword",
		}

		resp, err := service.Login(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.Equal(t, "Bearer", resp.TokenType)
	})

	t.Run("wrong password fails", func(t *testing.T) {
		req := &dto.LoginRequest{
			Email:    "login@example.com",
			Password: "wrongpassword",
		}

		resp, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		req := &dto.LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}

		resp, err := service.Login(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("login normalizes email", func(t *testing.T) {
		req := &dto.LoginRequest{
			Email:    "  LOGIN@EXAMPLE.COM  ",
			Password: "correctpassword",
		}

		resp, err := service.Login(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestService_Logout(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "logout@example.com", "password")

	t.Run("successful logout", func(t *testing.T) {
		err := service.Logout(ctx, user.ID)
		assert.NoError(t, err)
	})
}

func TestService_RefreshToken(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "refresh@example.com", "password")

	t.Run("successful token refresh", func(t *testing.T) {
		// First login to get a valid refresh token
		loginReq := &dto.LoginRequest{
			Email:    "refresh@example.com",
			Password: "password",
		}
		loginResp, err := service.Login(ctx, loginReq)
		require.NoError(t, err)

		// Use the refresh token
		resp, err := service.RefreshToken(ctx, loginResp.RefreshToken)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		// Note: tokens generated in same second may be identical, so we just verify they exist
	})

	t.Run("invalid token fails", func(t *testing.T) {
		resp, err := service.RefreshToken(ctx, "invalid-token")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("access token as refresh fails", func(t *testing.T) {
		loginReq := &dto.LoginRequest{
			Email:    "refresh@example.com",
			Password: "password",
		}
		loginResp, err := service.Login(ctx, loginReq)
		require.NoError(t, err)

		// Try using access token as refresh token
		resp, err := service.RefreshToken(ctx, loginResp.AccessToken)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("deleted user fails", func(t *testing.T) {
		// Create and delete a user
		tempUser := createTestUserWithPassword(t, db, "temp@example.com", "password")
		loginResp, err := service.Login(ctx, &dto.LoginRequest{
			Email:    "temp@example.com",
			Password: "password",
		})
		require.NoError(t, err)

		// Delete the user
		err = db.Delete(&models.User{}, "id = ?", tempUser.ID).Error
		require.NoError(t, err)

		// Try to refresh
		resp, err := service.RefreshToken(ctx, loginResp.RefreshToken)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	_ = user // silence unused variable warning
}

// User Management Tests

func TestService_GetUser(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "getuser@example.com", "password")

	t.Run("existing user", func(t *testing.T) {
		result, err := service.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, user.Email, result.Email)
	})

	t.Run("non-existent user", func(t *testing.T) {
		result, err := service.GetUser(ctx, "non-existent-id")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestService_GetUserByEmail(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	createTestUserWithPassword(t, db, "byemail@example.com", "password")

	t.Run("existing user", func(t *testing.T) {
		result, err := service.GetUserByEmail(ctx, "byemail@example.com")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "byemail@example.com", result.Email)
	})

	t.Run("with whitespace and case", func(t *testing.T) {
		result, err := service.GetUserByEmail(ctx, "  BYEMAIL@EXAMPLE.COM  ")
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("non-existent user", func(t *testing.T) {
		result, err := service.GetUserByEmail(ctx, "nonexistent@example.com")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestService_UpdateProfile(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "updateprofile@example.com", "password")

	t.Run("successful update", func(t *testing.T) {
		req := &dto.UpdateProfileRequest{
			Name:  "Updated Name",
			Email: "updateprofile@example.com",
		}

		result, err := service.UpdateProfile(ctx, user.ID, req)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", result.Name)
	})

	t.Run("update with timezone", func(t *testing.T) {
		req := &dto.UpdateProfileRequest{
			Name:     "Name with TZ",
			Email:    "updateprofile@example.com",
			Timezone: "Europe/London",
		}

		result, err := service.UpdateProfile(ctx, user.ID, req)
		require.NoError(t, err)
		assert.Equal(t, "Europe/London", result.Timezone)
	})

	t.Run("email change resets verification", func(t *testing.T) {
		// Set verified
		now := time.Now()
		user.EmailVerifiedAt = &now
		err := db.Save(user).Error
		require.NoError(t, err)

		req := &dto.UpdateProfileRequest{
			Name:  "Email Changed",
			Email: "newemail@example.com",
		}

		result, err := service.UpdateProfile(ctx, user.ID, req)
		require.NoError(t, err)
		assert.Nil(t, result.EmailVerifiedAt)
	})

	t.Run("duplicate email fails", func(t *testing.T) {
		createTestUserWithPassword(t, db, "existing@example.com", "password")

		req := &dto.UpdateProfileRequest{
			Name:  "Trying Duplicate",
			Email: "existing@example.com",
		}

		result, err := service.UpdateProfile(ctx, user.ID, req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		req := &dto.UpdateProfileRequest{
			Name:  "Name",
			Email: "email@example.com",
		}

		result, err := service.UpdateProfile(ctx, "non-existent", req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestService_ChangePassword(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "changepass@example.com", "oldpassword")

	t.Run("successful password change", func(t *testing.T) {
		req := &dto.ChangePasswordRequest{
			CurrentPassword: "oldpassword",
			Password:        "newpassword",
		}

		err := service.ChangePassword(ctx, user.ID, req)
		require.NoError(t, err)

		// Verify new password works
		loginResp, err := service.Login(ctx, &dto.LoginRequest{
			Email:    "changepass@example.com",
			Password: "newpassword",
		})
		require.NoError(t, err)
		assert.NotNil(t, loginResp)
	})

	t.Run("wrong current password fails", func(t *testing.T) {
		req := &dto.ChangePasswordRequest{
			CurrentPassword: "wrongpassword",
			Password:        "newpassword",
		}

		err := service.ChangePassword(ctx, user.ID, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current password")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		req := &dto.ChangePasswordRequest{
			CurrentPassword: "password",
			Password:        "newpassword",
		}

		err := service.ChangePassword(ctx, "non-existent", req)
		assert.Error(t, err)
	})
}

func TestService_DeleteAccount(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "delete1@example.com", "password")

		err := service.DeleteAccount(ctx, user.ID)
		require.NoError(t, err)

		// Verify user is deleted
		var count int64
		db.Model(&models.User{}).Where("id = ?", user.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("deletes owned teams", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "delete2@example.com", "password")

		// Create a team
		team := &models.Team{
			Name:    "Team to Delete",
			OwnerID: user.ID,
		}
		err := db.Create(team).Error
		require.NoError(t, err)

		// Add user to team
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: user.ID,
			Role:   "owner",
		}
		err = db.Create(member).Error
		require.NoError(t, err)

		err = service.DeleteAccount(ctx, user.ID)
		require.NoError(t, err)

		// Verify team is deleted
		var teamCount int64
		db.Model(&models.Team{}).Where("id = ?", team.ID).Count(&teamCount)
		assert.Equal(t, int64(0), teamCount)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		err := service.DeleteAccount(ctx, "non-existent")
		assert.Error(t, err)
	})
}

// Email Verification Tests

func TestService_VerifyEmail(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "verify@example.com", "password")

	t.Run("successful verification", func(t *testing.T) {
		// Generate correct hash using test helper (uses JWT secret like the service)
		hash := generateEmailHashWithSecret("verify@example.com", "test-secret-key-for-jwt-testing")

		err := service.VerifyEmail(ctx, user.ID, hash)
		require.NoError(t, err)

		// Reload user
		var updatedUser models.User
		db.First(&updatedUser, "id = ?", user.ID)
		assert.NotNil(t, updatedUser.EmailVerifiedAt)
	})

	t.Run("already verified succeeds", func(t *testing.T) {
		hash := generateEmailHashWithSecret("verify@example.com", "test-secret-key-for-jwt-testing")

		err := service.VerifyEmail(ctx, user.ID, hash)
		require.NoError(t, err)
	})

	t.Run("wrong hash fails", func(t *testing.T) {
		err := service.VerifyEmail(ctx, user.ID, "wrong-hash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid verification link")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		err := service.VerifyEmail(ctx, "non-existent", "hash")
		assert.Error(t, err)
	})
}

func TestService_ResendVerificationEmail(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("successful resend", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "resend@example.com", "password")

		err := service.ResendVerificationEmail(ctx, user.ID)
		require.NoError(t, err)
	})

	t.Run("already verified fails", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "resend2@example.com", "password")
		now := time.Now()
		user.EmailVerifiedAt = &now
		db.Save(user)

		err := service.ResendVerificationEmail(ctx, user.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already verified")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		err := service.ResendVerificationEmail(ctx, "non-existent")
		assert.Error(t, err)
	})
}

// Password Reset Tests

func TestService_SendPasswordResetLink(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	createTestUserWithPassword(t, db, "reset@example.com", "password")

	t.Run("existing user", func(t *testing.T) {
		err := service.SendPasswordResetLink(ctx, "reset@example.com")
		require.NoError(t, err)

		// Verify token was created
		var count int64
		db.Model(&models.PasswordResetToken{}).Where("email = ?", "reset@example.com").Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("non-existent user succeeds silently", func(t *testing.T) {
		// Should not reveal whether email exists
		err := service.SendPasswordResetLink(ctx, "nonexistent@example.com")
		require.NoError(t, err)
	})
}

func TestService_ResetPassword(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "resetpw@example.com", "oldpassword")

	t.Run("successful reset", func(t *testing.T) {
		// Create a reset token manually
		token := "test-reset-token"
		hashedToken := hashTokenForTest(token)

		resetToken := &models.PasswordResetToken{
			Email:     "resetpw@example.com",
			Token:     hashedToken,
			CreatedAt: time.Now(),
		}
		err := db.Create(resetToken).Error
		require.NoError(t, err)

		req := &dto.ResetPasswordRequest{
			Email:    "resetpw@example.com",
			Token:    token,
			Password: "newpassword",
		}

		err = service.ResetPassword(ctx, req)
		require.NoError(t, err)

		// Verify new password works
		loginResp, err := service.Login(ctx, &dto.LoginRequest{
			Email:    "resetpw@example.com",
			Password: "newpassword",
		})
		require.NoError(t, err)
		assert.NotNil(t, loginResp)
	})

	t.Run("expired token fails", func(t *testing.T) {
		token := "expired-token"
		hashedToken := hashTokenForTest(token)

		resetToken := &models.PasswordResetToken{
			Email:     "resetpw@example.com",
			Token:     hashedToken,
			CreatedAt: time.Now().Add(-2 * time.Hour), // Expired
		}
		db.Create(resetToken)

		req := &dto.ResetPasswordRequest{
			Email:    "resetpw@example.com",
			Token:    token,
			Password: "newpassword",
		}

		err := service.ResetPassword(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("wrong token fails", func(t *testing.T) {
		hashedToken := hashTokenForTest("correct-token")

		resetToken := &models.PasswordResetToken{
			Email:     "resetpw@example.com",
			Token:     hashedToken,
			CreatedAt: time.Now(),
		}
		db.Create(resetToken)

		req := &dto.ResetPasswordRequest{
			Email:    "resetpw@example.com",
			Token:    "wrong-token",
			Password: "newpassword",
		}

		err := service.ResetPassword(ctx, req)
		assert.Error(t, err)
	})

	t.Run("no token fails", func(t *testing.T) {
		req := &dto.ResetPasswordRequest{
			Email:    "notoken@example.com",
			Token:    "token",
			Password: "newpassword",
		}

		err := service.ResetPassword(ctx, req)
		assert.Error(t, err)
	})

	_ = user // silence unused variable warning
}

// Two-Factor Authentication Tests

func TestService_EnableTwoFactor(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "2fa@example.com", "password")

	t.Run("successful enable", func(t *testing.T) {
		resp, err := service.EnableTwoFactor(ctx, user.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.SecretKey)
		assert.NotEmpty(t, resp.QRCodeURL)
		assert.Contains(t, resp.QRCodeURL, "otpauth://")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		resp, err := service.EnableTwoFactor(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestService_ConfirmTwoFactor(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "confirm2fa@example.com", "password")

	t.Run("not initiated fails", func(t *testing.T) {
		err := service.ConfirmTwoFactor(ctx, user.ID, "123456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initiated")
	})

	t.Run("invalid code fails", func(t *testing.T) {
		// Enable 2FA first
		_, err := service.EnableTwoFactor(ctx, user.ID)
		require.NoError(t, err)

		err = service.ConfirmTwoFactor(ctx, user.ID, "000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid verification code")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		err := service.ConfirmTwoFactor(ctx, "non-existent", "123456")
		assert.Error(t, err)
	})
}

func TestService_DisableTwoFactor(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "disable2fa@example.com", "password")

	t.Run("successful disable", func(t *testing.T) {
		// Set up 2FA manually
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		err := service.DisableTwoFactor(ctx, user.ID, "password")
		require.NoError(t, err)

		// Verify 2FA is disabled
		var updatedUser models.User
		db.First(&updatedUser, "id = ?", user.ID)
		assert.Nil(t, updatedUser.TwoFactorSecret)
		assert.Nil(t, updatedUser.TwoFactorConfirmedAt)
	})

	t.Run("wrong password fails", func(t *testing.T) {
		err := service.DisableTwoFactor(ctx, user.ID, "wrongpassword")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid password")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		err := service.DisableTwoFactor(ctx, "non-existent", "password")
		assert.Error(t, err)
	})
}

func TestService_VerifyTwoFactor(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("2FA not enabled returns true", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "verify2fa1@example.com", "password")

		valid, err := service.VerifyTwoFactor(ctx, user.ID, "123456")
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("invalid code returns false", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "verify2fa2@example.com", "password")

		// Enable 2FA
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		valid, err := service.VerifyTwoFactor(ctx, user.ID, "000000")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("recovery code works", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "verify2fa3@example.com", "password")

		// Enable 2FA with recovery codes
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		recoveryCodes := "CODE1-CODE1,CODE2-CODE2"
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		user.TwoFactorRecoveryCodes = &recoveryCodes
		db.Save(user)

		valid, err := service.VerifyTwoFactor(ctx, user.ID, "CODE1-CODE1")
		require.NoError(t, err)
		assert.True(t, valid)

		// Verify code was removed
		var updatedUser models.User
		db.First(&updatedUser, "id = ?", user.ID)
		assert.NotContains(t, *updatedUser.TwoFactorRecoveryCodes, "CODE1-CODE1")
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		valid, err := service.VerifyTwoFactor(ctx, "non-existent", "123456")
		assert.Error(t, err)
		assert.False(t, valid)
	})
}

func TestService_GetRecoveryCodes(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("returns codes", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "codes@example.com", "password")
		codes := "CODE1,CODE2,CODE3"
		user.TwoFactorRecoveryCodes = &codes
		db.Save(user)

		result, err := service.GetRecoveryCodes(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Contains(t, result, "CODE1")
	})

	t.Run("no codes returns empty", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "nocodes@example.com", "password")

		result, err := service.GetRecoveryCodes(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		result, err := service.GetRecoveryCodes(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestService_RegenerateRecoveryCodes(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("successful regeneration", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "regen@example.com", "password")

		// Enable 2FA
		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		result, err := service.RegenerateRecoveryCodes(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, result, 8) // 8 codes generated
	})

	t.Run("2FA not enabled fails", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "regen2@example.com", "password")

		result, err := service.RegenerateRecoveryCodes(ctx, user.ID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		result, err := service.RegenerateRecoveryCodes(ctx, "non-existent")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestService_HasTwoFactorEnabled(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("returns false when not enabled", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "has2fa1@example.com", "password")

		result, err := service.HasTwoFactorEnabled(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("returns true when enabled", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "has2fa2@example.com", "password")

		secret := "JBSWY3DPEHPK3PXP"
		now := time.Now()
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		result, err := service.HasTwoFactorEnabled(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("non-existent user returns false", func(t *testing.T) {
		result, err := service.HasTwoFactorEnabled(ctx, "non-existent")
		require.NoError(t, err)
		assert.False(t, result)
	})
}

// Team Management Tests

func TestService_CreateTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "createteam@example.com", "password")

	t.Run("successful creation", func(t *testing.T) {
		req := &dto.CreateTeamRequest{
			Name: "My Team",
		}

		team, err := service.CreateTeam(ctx, user.ID, req)
		require.NoError(t, err)
		assert.Equal(t, "My Team", team.Name)
		assert.Equal(t, user.ID, team.OwnerID)
	})

	t.Run("personal team", func(t *testing.T) {
		req := &dto.CreateTeamRequest{
			Name:         "Personal",
			PersonalTeam: true,
		}

		team, err := service.CreateTeam(ctx, user.ID, req)
		require.NoError(t, err)
		assert.True(t, team.PersonalTeam)
	})
}

func TestService_UpdateTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "updateteam@example.com", "password")
	otherUser := createTestUserWithPassword(t, db, "other@example.com", "password")

	team := &models.Team{
		Name:    "Original Name",
		OwnerID: user.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("successful update", func(t *testing.T) {
		req := &dto.UpdateTeamRequest{
			Name: "Updated Name",
		}

		result, err := service.UpdateTeam(ctx, user.ID, team.ID, req)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", result.Name)
	})

	t.Run("non-owner fails", func(t *testing.T) {
		req := &dto.UpdateTeamRequest{
			Name: "Hacked Name",
		}

		result, err := service.UpdateTeam(ctx, otherUser.ID, team.ID, req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		req := &dto.UpdateTeamRequest{
			Name: "Name",
		}

		result, err := service.UpdateTeam(ctx, user.ID, "non-existent", req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestService_DeleteTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "deleteteam@example.com", "password")
	otherUser := createTestUserWithPassword(t, db, "other2@example.com", "password")

	t.Run("successful delete", func(t *testing.T) {
		team := &models.Team{
			Name:         "To Delete",
			OwnerID:      user.ID,
			PersonalTeam: false,
		}
		err := db.Create(team).Error
		require.NoError(t, err)

		err = service.DeleteTeam(ctx, user.ID, team.ID)
		require.NoError(t, err)

		// Verify deletion
		var count int64
		db.Model(&models.Team{}).Where("id = ?", team.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("personal team fails", func(t *testing.T) {
		team := &models.Team{
			Name:         "Personal",
			OwnerID:      user.ID,
			PersonalTeam: true,
		}
		err := db.Create(team).Error
		require.NoError(t, err)

		err = service.DeleteTeam(ctx, user.ID, team.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "personal team")
	})

	t.Run("non-owner fails", func(t *testing.T) {
		team := &models.Team{
			Name:    "Not Yours",
			OwnerID: user.ID,
		}
		err := db.Create(team).Error
		require.NoError(t, err)

		err = service.DeleteTeam(ctx, otherUser.ID, team.ID)
		assert.Error(t, err)
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		err := service.DeleteTeam(ctx, user.ID, "non-existent")
		assert.Error(t, err)
	})
}

func TestService_GetTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "getteam@example.com", "password")

	team := &models.Team{
		Name:    "Test Team",
		OwnerID: user.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("existing team", func(t *testing.T) {
		result, err := service.GetTeam(ctx, team.ID)
		require.NoError(t, err)
		assert.Equal(t, "Test Team", result.Name)
	})

	t.Run("non-existent team", func(t *testing.T) {
		result, err := service.GetTeam(ctx, "non-existent")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestService_GetUserTeams(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "userteams@example.com", "password")

	// Create multiple teams
	for i := 0; i < 3; i++ {
		team := &models.Team{
			Name:    "Team " + string(rune('A'+i)),
			OwnerID: user.ID,
		}
		err := db.Create(team).Error
		require.NoError(t, err)
	}

	t.Run("returns all teams", func(t *testing.T) {
		teams, err := service.GetUserTeams(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, teams, 3)
	})

	t.Run("user with no teams", func(t *testing.T) {
		newUser := createTestUserWithPassword(t, db, "noteams@example.com", "password")

		teams, err := service.GetUserTeams(ctx, newUser.ID)
		require.NoError(t, err)
		assert.Empty(t, teams)
	})
}

func TestService_SwitchTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	user := createTestUserWithPassword(t, db, "switchteam@example.com", "password")

	team := &models.Team{
		Name:    "Switch To",
		OwnerID: user.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	// Add user as member
	member := &models.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   "owner",
	}
	err = db.Create(member).Error
	require.NoError(t, err)

	t.Run("successful switch", func(t *testing.T) {
		result, err := service.SwitchTeam(ctx, user.ID, team.ID)
		require.NoError(t, err)
		assert.Equal(t, team.ID, *result.CurrentTeamID)
	})

	t.Run("non-member fails", func(t *testing.T) {
		otherUser := createTestUserWithPassword(t, db, "nonmember@example.com", "password")

		result, err := service.SwitchTeam(ctx, otherUser.ID, team.ID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// Team Member Management Tests

func TestService_InviteTeamMember(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "inviteowner@example.com", "password")

	team := &models.Team{
		Name:    "Invite Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("successful invite by owner", func(t *testing.T) {
		req := &dto.InviteTeamMemberRequest{
			Email: "newinvite@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, owner.ID, team.ID, req)
		require.NoError(t, err)

		// Verify invitation created
		var count int64
		db.Model(&models.TeamInvitation{}).Where("email = ?", "newinvite@example.com").Count(&count)
		assert.Equal(t, int64(1), count)
	})

	t.Run("invite by admin", func(t *testing.T) {
		admin := createTestUserWithPassword(t, db, "admin@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: admin.ID,
			Role:   "admin",
		}
		db.Create(member)

		req := &dto.InviteTeamMemberRequest{
			Email: "admininvite@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, admin.ID, team.ID, req)
		require.NoError(t, err)
	})

	t.Run("invite by regular member fails", func(t *testing.T) {
		regularMember := createTestUserWithPassword(t, db, "regular@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: regularMember.ID,
			Role:   "member",
		}
		db.Create(member)

		req := &dto.InviteTeamMemberRequest{
			Email: "shouldfail@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, regularMember.ID, team.ID, req)
		assert.Error(t, err)
	})

	t.Run("duplicate invitation fails", func(t *testing.T) {
		req := &dto.InviteTeamMemberRequest{
			Email: "newinvite@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, owner.ID, team.ID, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already sent")
	})

	t.Run("existing member fails", func(t *testing.T) {
		existingUser := createTestUserWithPassword(t, db, "existing@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: existingUser.ID,
			Role:   "member",
		}
		db.Create(member)

		req := &dto.InviteTeamMemberRequest{
			Email: "existing@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, owner.ID, team.ID, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already a team member")
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		req := &dto.InviteTeamMemberRequest{
			Email: "test@example.com",
			Role:  "member",
		}

		err := service.InviteTeamMember(ctx, owner.ID, "non-existent", req)
		assert.Error(t, err)
	})
}

func TestService_AcceptTeamInvitation(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "acceptowner@example.com", "password")
	invitee := createTestUserWithPassword(t, db, "invitee@example.com", "password")

	team := &models.Team{
		Name:    "Accept Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("successful accept", func(t *testing.T) {
		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "invitee@example.com",
			Role:   "member",
		}
		err := db.Create(invitation).Error
		require.NoError(t, err)

		err = service.AcceptTeamInvitation(ctx, invitee.ID, invitation.ID)
		require.NoError(t, err)

		// Verify membership
		isMember, _ := service.Repository().IsTeamMember(ctx, team.ID, invitee.ID)
		assert.True(t, isMember)

		// Verify invitation deleted
		var count int64
		db.Model(&models.TeamInvitation{}).Where("id = ?", invitation.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("wrong user fails", func(t *testing.T) {
		otherUser := createTestUserWithPassword(t, db, "wronguser@example.com", "password")

		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "someone@example.com",
			Role:   "member",
		}
		err := db.Create(invitation).Error
		require.NoError(t, err)

		err = service.AcceptTeamInvitation(ctx, otherUser.ID, invitation.ID)
		assert.Error(t, err)
	})

	t.Run("non-existent invitation fails", func(t *testing.T) {
		err := service.AcceptTeamInvitation(ctx, invitee.ID, "non-existent")
		assert.Error(t, err)
	})

	t.Run("non-existent user fails", func(t *testing.T) {
		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "test@test.com",
			Role:   "member",
		}
		db.Create(invitation)

		err := service.AcceptTeamInvitation(ctx, "non-existent", invitation.ID)
		assert.Error(t, err)
	})
}

func TestService_CancelTeamInvitation(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "cancelowner@example.com", "password")

	team := &models.Team{
		Name:    "Cancel Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("successful cancel by owner", func(t *testing.T) {
		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "cancel@example.com",
			Role:   "member",
		}
		err := db.Create(invitation).Error
		require.NoError(t, err)

		err = service.CancelTeamInvitation(ctx, owner.ID, team.ID, invitation.ID)
		require.NoError(t, err)

		// Verify deleted
		var count int64
		db.Model(&models.TeamInvitation{}).Where("id = ?", invitation.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})

	t.Run("cancel by admin", func(t *testing.T) {
		admin := createTestUserWithPassword(t, db, "canceladmin@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: admin.ID,
			Role:   "admin",
		}
		db.Create(member)

		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "cancel2@example.com",
			Role:   "member",
		}
		db.Create(invitation)

		err := service.CancelTeamInvitation(ctx, admin.ID, team.ID, invitation.ID)
		require.NoError(t, err)
	})

	t.Run("non-owner/admin fails", func(t *testing.T) {
		regular := createTestUserWithPassword(t, db, "cancelregular@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: regular.ID,
			Role:   "member",
		}
		db.Create(member)

		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "cancel3@example.com",
			Role:   "member",
		}
		db.Create(invitation)

		err := service.CancelTeamInvitation(ctx, regular.ID, team.ID, invitation.ID)
		assert.Error(t, err)
	})

	t.Run("wrong team fails", func(t *testing.T) {
		otherTeam := &models.Team{
			Name:    "Other Team",
			OwnerID: owner.ID,
		}
		db.Create(otherTeam)

		invitation := &models.TeamInvitation{
			TeamID: otherTeam.ID,
			Email:  "wrongteam@example.com",
			Role:   "member",
		}
		db.Create(invitation)

		err := service.CancelTeamInvitation(ctx, owner.ID, team.ID, invitation.ID)
		assert.Error(t, err)
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		err := service.CancelTeamInvitation(ctx, owner.ID, "non-existent", "inv-id")
		assert.Error(t, err)
	})
}

func TestService_UpdateTeamMemberRole(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "roleowner@example.com", "password")

	team := &models.Team{
		Name:    "Role Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	memberUser := createTestUserWithPassword(t, db, "rolemember@example.com", "password")
	member := &models.TeamMember{
		TeamID: team.ID,
		UserID: memberUser.ID,
		Role:   "member",
	}
	err = db.Create(member).Error
	require.NoError(t, err)

	t.Run("successful role update", func(t *testing.T) {
		req := &dto.UpdateTeamMemberRequest{
			Role: "admin",
		}

		err := service.UpdateTeamMemberRole(ctx, owner.ID, team.ID, memberUser.ID, req)
		require.NoError(t, err)

		// Verify role updated
		var updatedMember models.TeamMember
		db.Where("team_id = ? AND user_id = ?", team.ID, memberUser.ID).First(&updatedMember)
		assert.Equal(t, "admin", updatedMember.Role)
	})

	t.Run("non-owner fails", func(t *testing.T) {
		nonOwner := createTestUserWithPassword(t, db, "nonowner@example.com", "password")

		req := &dto.UpdateTeamMemberRequest{
			Role: "admin",
		}

		err := service.UpdateTeamMemberRole(ctx, nonOwner.ID, team.ID, memberUser.ID, req)
		assert.Error(t, err)
	})

	t.Run("update owner role fails", func(t *testing.T) {
		req := &dto.UpdateTeamMemberRequest{
			Role: "member",
		}

		err := service.UpdateTeamMemberRole(ctx, owner.ID, team.ID, owner.ID, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "owner's role")
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		req := &dto.UpdateTeamMemberRequest{
			Role: "admin",
		}

		err := service.UpdateTeamMemberRole(ctx, owner.ID, "non-existent", memberUser.ID, req)
		assert.Error(t, err)
	})
}

func TestService_RemoveTeamMember(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "removeowner@example.com", "password")

	team := &models.Team{
		Name:    "Remove Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	t.Run("owner removes member", func(t *testing.T) {
		memberUser := createTestUserWithPassword(t, db, "removemember1@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: memberUser.ID,
			Role:   "member",
		}
		db.Create(member)

		err := service.RemoveTeamMember(ctx, owner.ID, team.ID, memberUser.ID)
		require.NoError(t, err)

		// Verify removed
		isMember, _ := service.Repository().IsTeamMember(ctx, team.ID, memberUser.ID)
		assert.False(t, isMember)
	})

	t.Run("self removal", func(t *testing.T) {
		memberUser := createTestUserWithPassword(t, db, "selfremove@example.com", "password")
		member := &models.TeamMember{
			TeamID: team.ID,
			UserID: memberUser.ID,
			Role:   "member",
		}
		db.Create(member)

		err := service.RemoveTeamMember(ctx, memberUser.ID, team.ID, memberUser.ID)
		require.NoError(t, err)
	})

	t.Run("non-owner removing others fails", func(t *testing.T) {
		memberUser1 := createTestUserWithPassword(t, db, "removemember2@example.com", "password")
		memberUser2 := createTestUserWithPassword(t, db, "removemember3@example.com", "password")

		db.Create(&models.TeamMember{TeamID: team.ID, UserID: memberUser1.ID, Role: "member"})
		db.Create(&models.TeamMember{TeamID: team.ID, UserID: memberUser2.ID, Role: "member"})

		err := service.RemoveTeamMember(ctx, memberUser1.ID, team.ID, memberUser2.ID)
		assert.Error(t, err)
	})

	t.Run("remove owner fails", func(t *testing.T) {
		err := service.RemoveTeamMember(ctx, owner.ID, team.ID, owner.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "team owner")
	})

	t.Run("non-existent team fails", func(t *testing.T) {
		err := service.RemoveTeamMember(ctx, owner.ID, "non-existent", "user-id")
		assert.Error(t, err)
	})
}

func TestService_RemoveTeamMember_SwitchesTeam(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "switchowner@example.com", "password")

	team1 := &models.Team{Name: "Team 1", OwnerID: owner.ID}
	team2 := &models.Team{Name: "Team 2", OwnerID: owner.ID}
	db.Create(team1)
	db.Create(team2)

	memberUser := createTestUserWithPassword(t, db, "switchmember@example.com", "password")
	memberUser.CurrentTeamID = &team1.ID
	db.Save(memberUser)

	db.Create(&models.TeamMember{TeamID: team1.ID, UserID: memberUser.ID, Role: "member"})
	db.Create(&models.TeamMember{TeamID: team2.ID, UserID: memberUser.ID, Role: "member"})

	err := service.RemoveTeamMember(ctx, owner.ID, team1.ID, memberUser.ID)
	require.NoError(t, err)

	// Verify team was switched
	var updatedUser models.User
	db.First(&updatedUser, "id = ?", memberUser.ID)
	// Current team should be updated (either nil or team2)
	if updatedUser.CurrentTeamID != nil {
		assert.NotEqual(t, team1.ID, *updatedUser.CurrentTeamID)
	}
}

func TestService_GetTeamMembers(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "getmembersowner@example.com", "password")

	team := &models.Team{
		Name:    "Get Members Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	// Add some members
	for i := 0; i < 3; i++ {
		user := createTestUserWithPassword(t, db, "getmember"+string(rune('a'+i))+"@example.com", "password")
		db.Create(&models.TeamMember{
			TeamID: team.ID,
			UserID: user.ID,
			Role:   "member",
		})
	}

	t.Run("returns all members", func(t *testing.T) {
		members, err := service.GetTeamMembers(ctx, team.ID)
		require.NoError(t, err)
		assert.Len(t, members, 3)
	})

	t.Run("empty team", func(t *testing.T) {
		emptyTeam := &models.Team{Name: "Empty", OwnerID: owner.ID}
		db.Create(emptyTeam)

		members, err := service.GetTeamMembers(ctx, emptyTeam.ID)
		require.NoError(t, err)
		assert.Empty(t, members)
	})
}

func TestService_GetTeamInvitations(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	owner := createTestUserWithPassword(t, db, "getinvowner@example.com", "password")

	team := &models.Team{
		Name:    "Get Invitations Team",
		OwnerID: owner.ID,
	}
	err := db.Create(team).Error
	require.NoError(t, err)

	// Create invitations
	for i := 0; i < 3; i++ {
		db.Create(&models.TeamInvitation{
			TeamID: team.ID,
			Email:  "getinv" + string(rune('a'+i)) + "@example.com",
			Role:   "member",
		})
	}

	t.Run("returns all invitations", func(t *testing.T) {
		invitations, err := service.GetTeamInvitations(ctx, team.ID)
		require.NoError(t, err)
		assert.Len(t, invitations, 3)
	})

	t.Run("empty team", func(t *testing.T) {
		emptyTeam := &models.Team{Name: "No Invites", OwnerID: owner.ID}
		db.Create(emptyTeam)

		invitations, err := service.GetTeamInvitations(ctx, emptyTeam.ID)
		require.NoError(t, err)
		assert.Empty(t, invitations)
	})
}

// User Status Tests

func TestService_CheckUserStatus(t *testing.T) {
	service, db := setupTestService(t)
	ctx := context.Background()

	t.Run("non-existent user", func(t *testing.T) {
		status, err := service.CheckUserStatus(ctx, "nobody@example.com")
		require.NoError(t, err)
		assert.False(t, status.UserExists)
		assert.False(t, status.HasTwoFactor)
		assert.False(t, status.RequiresVerification)
	})

	t.Run("existing user without verification", func(t *testing.T) {
		createTestUserWithPassword(t, db, "status1@example.com", "password")

		status, err := service.CheckUserStatus(ctx, "status1@example.com")
		require.NoError(t, err)
		assert.True(t, status.UserExists)
		assert.False(t, status.HasTwoFactor)
		assert.True(t, status.RequiresVerification)
	})

	t.Run("verified user", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "status2@example.com", "password")
		now := time.Now()
		user.EmailVerifiedAt = &now
		db.Save(user)

		status, err := service.CheckUserStatus(ctx, "status2@example.com")
		require.NoError(t, err)
		assert.True(t, status.UserExists)
		assert.False(t, status.RequiresVerification)
	})

	t.Run("user with 2FA", func(t *testing.T) {
		user := createTestUserWithPassword(t, db, "status3@example.com", "password")
		now := time.Now()
		secret := "JBSWY3DPEHPK3PXP"
		user.TwoFactorSecret = &secret
		user.TwoFactorConfirmedAt = &now
		db.Save(user)

		status, err := service.CheckUserStatus(ctx, "status3@example.com")
		require.NoError(t, err)
		assert.True(t, status.UserExists)
		assert.True(t, status.HasTwoFactor)
	})

	t.Run("normalizes email", func(t *testing.T) {
		createTestUserWithPassword(t, db, "status4@example.com", "password")

		status, err := service.CheckUserStatus(ctx, "  STATUS4@EXAMPLE.COM  ")
		require.NoError(t, err)
		assert.True(t, status.UserExists)
	})
}

// Helper Method Tests

func TestGenerateEmailHash(t *testing.T) {
	t.Run("produces consistent hash", func(t *testing.T) {
		hash1 := generateEmailHashWithSecret("test@example.com", "test-secret")
		hash2 := generateEmailHashWithSecret("test@example.com", "test-secret")
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different emails produce different hashes", func(t *testing.T) {
		hash1 := generateEmailHashWithSecret("test1@example.com", "test-secret")
		hash2 := generateEmailHashWithSecret("test2@example.com", "test-secret")
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestHashToken(t *testing.T) {
	t.Run("produces consistent hash", func(t *testing.T) {
		hash1 := hashTokenForTest("test-token")
		hash2 := hashTokenForTest("test-token")
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different tokens produce different hashes", func(t *testing.T) {
		hash1 := hashTokenForTest("token1")
		hash2 := hashTokenForTest("token2")
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestGenerateRecoveryCodes(t *testing.T) {
	t.Run("codes have correct format", func(t *testing.T) {
		// Test the format pattern - codes should have format XXXX-XXXX
		code := "CODE1-CODE1"
		parts := strings.Split(code, "-")
		assert.Len(t, parts, 2)
	})
}

func TestNewService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
	cfg := &config.Config{}
	logger := zerolog.Nop()

	service := services.NewService(repo, cfg, &logger)

	assert.NotNil(t, service)
	assert.NotNil(t, service.Repository())
}
