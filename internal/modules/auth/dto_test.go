package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// Normalize Tests

func TestRegisterRequest_Normalize(t *testing.T) {
	req := &dto.RegisterRequest{
		Name:  "  John Doe  ",
		Email: "  JOHN@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "John Doe", req.Name)
	assert.Equal(t, "john@example.com", req.Email)
}

func TestLoginRequest_Normalize(t *testing.T) {
	req := &dto.LoginRequest{
		Email: "  LOGIN@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "login@example.com", req.Email)
}

func TestUpdateProfileRequest_Normalize(t *testing.T) {
	req := &dto.UpdateProfileRequest{
		Name:  "  Jane Doe  ",
		Email: "  JANE@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "Jane Doe", req.Name)
	assert.Equal(t, "jane@example.com", req.Email)
}

func TestForgotPasswordRequest_Normalize(t *testing.T) {
	req := &dto.ForgotPasswordRequest{
		Email: "  FORGOT@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "forgot@example.com", req.Email)
}

func TestResetPasswordRequest_Normalize(t *testing.T) {
	req := &dto.ResetPasswordRequest{
		Email: "  RESET@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "reset@example.com", req.Email)
}

func TestInviteTeamMemberRequest_Normalize(t *testing.T) {
	req := &dto.InviteTeamMemberRequest{
		Email: "  INVITE@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "invite@example.com", req.Email)
}

func TestCheckUserStatusRequest_Normalize(t *testing.T) {
	req := &dto.CheckUserStatusRequest{
		Email: "  STATUS@EXAMPLE.COM  ",
	}

	req.Normalize()

	assert.Equal(t, "status@example.com", req.Email)
}

// ToUserResponse Tests

func TestToUserResponse(t *testing.T) {
	t.Run("basic user", func(t *testing.T) {
		user := &models.User{
			ID:       "user123",
			Name:     "Test User",
			Email:    "test@example.com",
			Timezone: "UTC",
		}

		resp := dto.ToUserResponse(user)

		assert.Equal(t, "user123", resp.ID)
		assert.Equal(t, "Test User", resp.Name)
		assert.Equal(t, "test@example.com", resp.Email)
		assert.Equal(t, "UTC", resp.Timezone)
		assert.Nil(t, resp.EmailVerifiedAt)
		assert.Nil(t, resp.CurrentTeamID)
		assert.Nil(t, resp.CurrentTeam)
		assert.False(t, resp.TwoFactorEnabled)
	})

	t.Run("user with verified email", func(t *testing.T) {
		now := time.Now()
		user := &models.User{
			ID:              "user123",
			Name:            "Test User",
			Email:           "test@example.com",
			EmailVerifiedAt: &now,
		}

		resp := dto.ToUserResponse(user)

		assert.NotNil(t, resp.EmailVerifiedAt)
	})

	t.Run("user with current team", func(t *testing.T) {
		teamID := "team123"
		team := &models.Team{
			ID:      teamID,
			Name:    "Test Team",
			OwnerID: "owner123",
		}
		user := &models.User{
			ID:            "user123",
			Name:          "Test User",
			Email:         "test@example.com",
			CurrentTeamID: &teamID,
			CurrentTeam:   team,
		}

		resp := dto.ToUserResponse(user)

		assert.NotNil(t, resp.CurrentTeamID)
		assert.NotNil(t, resp.CurrentTeam)
		assert.Equal(t, teamID, resp.CurrentTeam.ID)
	})

	t.Run("user with 2FA enabled", func(t *testing.T) {
		secret := "secret"
		now := time.Now()
		user := &models.User{
			ID:                   "user123",
			Name:                 "Test User",
			Email:                "test@example.com",
			TwoFactorSecret:      &secret,
			TwoFactorConfirmedAt: &now,
		}

		resp := dto.ToUserResponse(user)

		assert.True(t, resp.TwoFactorEnabled)
	})
}

// ToTeamResponse Tests

func TestToTeamResponse(t *testing.T) {
	team := models.Team{
		ID:           "team123",
		Name:         "Test Team",
		OwnerID:      "owner123",
		PersonalTeam: false,
	}

	resp := dto.ToTeamResponse(team)

	assert.Equal(t, "team123", resp.ID)
	assert.Equal(t, "Test Team", resp.Name)
	assert.Equal(t, "owner123", resp.OwnerID)
	assert.False(t, resp.PersonalTeam)
	assert.NotEmpty(t, resp.ImageURL)
}

func TestToTeamResponsePtr(t *testing.T) {
	t.Run("nil team", func(t *testing.T) {
		resp := dto.ToTeamResponsePtr(nil)
		assert.Nil(t, resp)
	})

	t.Run("valid team", func(t *testing.T) {
		team := &models.Team{
			ID:      "team123",
			Name:    "Test Team",
			OwnerID: "owner123",
		}

		resp := dto.ToTeamResponsePtr(team)

		assert.NotNil(t, resp)
		assert.Equal(t, "team123", resp.ID)
	})
}

// ToTeamMemberResponse Tests

func TestToTeamMemberResponse(t *testing.T) {
	user := &models.User{
		ID:    "user123",
		Name:  "Test User",
		Email: "test@example.com",
	}
	role := "admin"
	joinedAt := time.Now()

	resp := dto.ToTeamMemberResponse(user, role, joinedAt)

	assert.Equal(t, "user123", resp.ID)
	assert.Equal(t, "Test User", resp.Name)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.Equal(t, "admin", resp.Role)
	assert.NotEmpty(t, resp.JoinedAt)
	assert.NotEmpty(t, resp.AvatarURL)
}

// ToTeamInvitationResponse Tests

func TestToTeamInvitationResponse(t *testing.T) {
	t.Run("invitation without team", func(t *testing.T) {
		invitation := &models.TeamInvitation{
			ID:    "inv123",
			Email: "invite@example.com",
			Role:  "member",
		}

		resp := dto.ToTeamInvitationResponse(invitation)

		assert.Equal(t, "inv123", resp.ID)
		assert.Equal(t, "invite@example.com", resp.Email)
		assert.Equal(t, "member", resp.Role)
	})

	t.Run("invitation with team", func(t *testing.T) {
		team := &models.Team{
			ID:      "team123",
			Name:    "Test Team",
			OwnerID: "owner123",
		}
		invitation := &models.TeamInvitation{
			ID:    "inv123",
			Email: "invite@example.com",
			Role:  "member",
			Team:  team,
		}

		resp := dto.ToTeamInvitationResponse(invitation)

		assert.Equal(t, "inv123", resp.ID)
		assert.Equal(t, "team123", resp.Team.ID)
	})
}

// ToTeamsResponse Tests

func TestToTeamsResponse(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		teams := []models.Team{}

		resp := dto.ToTeamsResponse(teams)

		assert.Empty(t, resp)
	})

	t.Run("multiple teams", func(t *testing.T) {
		teams := []models.Team{
			{ID: "team1", Name: "Team 1", OwnerID: "owner1"},
			{ID: "team2", Name: "Team 2", OwnerID: "owner2"},
		}

		resp := dto.ToTeamsResponse(teams)

		assert.Len(t, resp, 2)
		assert.Equal(t, "team1", resp[0].ID)
		assert.Equal(t, "team2", resp[1].ID)
	})
}

// ToTeamMembersResponse Tests

func TestToTeamMembersResponse(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		members := []models.TeamMember{}

		resp := dto.ToTeamMembersResponse(members)

		assert.Empty(t, resp)
	})

	t.Run("members with users", func(t *testing.T) {
		user1 := &models.User{ID: "user1", Name: "User 1", Email: "user1@example.com"}
		user2 := &models.User{ID: "user2", Name: "User 2", Email: "user2@example.com"}
		members := []models.TeamMember{
			{ID: "m1", UserID: "user1", Role: "admin", User: user1},
			{ID: "m2", UserID: "user2", Role: "member", User: user2},
		}

		resp := dto.ToTeamMembersResponse(members)

		assert.Len(t, resp, 2)
		assert.Equal(t, "user1", resp[0].ID)
		assert.Equal(t, "admin", resp[0].Role)
		assert.Equal(t, "user2", resp[1].ID)
		assert.Equal(t, "member", resp[1].Role)
	})

	t.Run("member without user", func(t *testing.T) {
		members := []models.TeamMember{
			{ID: "m1", UserID: "user1", Role: "admin", User: nil},
		}

		resp := dto.ToTeamMembersResponse(members)

		assert.Len(t, resp, 1)
		// User is nil, so response fields will be empty
		assert.Equal(t, "", resp[0].ID)
	})
}

// ToTeamInvitationsResponse Tests

func TestToTeamInvitationsResponse(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		invitations := []models.TeamInvitation{}

		resp := dto.ToTeamInvitationsResponse(invitations)

		assert.Empty(t, resp)
	})

	t.Run("multiple invitations", func(t *testing.T) {
		invitations := []models.TeamInvitation{
			{ID: "inv1", Email: "a@example.com", Role: "admin"},
			{ID: "inv2", Email: "b@example.com", Role: "member"},
		}

		resp := dto.ToTeamInvitationsResponse(invitations)

		assert.Len(t, resp, 2)
		assert.Equal(t, "inv1", resp[0].ID)
		assert.Equal(t, "inv2", resp[1].ID)
	})
}
