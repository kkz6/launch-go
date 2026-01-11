package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
)

func setupTestRepository(t *testing.T) (*repositories.Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{}, &models.TeamInvitation{}, &models.PersonalAccessToken{}, &models.PasswordResetToken{})
	require.NoError(t, err)

	return repositories.NewRepository(db), db
}

func createTestUser(t *testing.T, repo *repositories.Repository, email string) *models.User {
	user := &models.User{
		Name:     "Test User",
		Email:    email,
		Password: "hashedpassword",
		Timezone: "UTC",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}

func createTestTeam(t *testing.T, repo *repositories.Repository, ownerID, name string) *models.Team {
	team := &models.Team{
		Name:    name,
		OwnerID: ownerID,
	}
	err := repo.CreateTeam(context.Background(), team)
	require.NoError(t, err)

	return team
}

// User Repository Tests

func TestRepository_CreateUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := &models.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "password",
	}

	err := repo.CreateUser(ctx, user)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)
}

func TestRepository_FindUserByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	t.Run("finds existing user", func(t *testing.T) {
		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("returns nil for non-existent user", func(t *testing.T) {
		found, err := repo.FindUserByID(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_FindUserByEmail(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	t.Run("finds existing user", func(t *testing.T) {
		found, err := repo.FindUserByEmail(ctx, user.Email)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, user.ID, found.ID)
	})

	t.Run("returns nil for non-existent email", func(t *testing.T) {
		found, err := repo.FindUserByEmail(ctx, "nonexistent@example.com")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_UpdateUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	user.Name = "Updated Name"

	err := repo.UpdateUser(ctx, user)
	assert.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.Name)
}

func TestRepository_DeleteUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	err := repo.DeleteUser(ctx, user.ID)
	assert.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	assert.NoError(t, err)
	assert.Nil(t, found)
}

func TestRepository_UserExistsByEmail(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	createTestUser(t, repo, "test@example.com")

	t.Run("returns true for existing email", func(t *testing.T) {
		exists, err := repo.UserExistsByEmail(ctx, "test@example.com")
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns false for non-existent email", func(t *testing.T) {
		exists, err := repo.UserExistsByEmail(ctx, "nonexistent@example.com")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestRepository_SetCurrentTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	err := repo.SetCurrentTeam(ctx, user.ID, team.ID)
	assert.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, team.ID, *found.CurrentTeamID)
}

func TestRepository_MarkEmailAsVerified(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	assert.Nil(t, user.EmailVerifiedAt)

	err := repo.MarkEmailAsVerified(ctx, user.ID)
	assert.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.EmailVerifiedAt)
}

// Team Repository Tests

func TestRepository_CreateTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := &models.Team{
		Name:    "Test Team",
		OwnerID: user.ID,
	}

	err := repo.CreateTeam(ctx, team)
	assert.NoError(t, err)
	assert.NotEmpty(t, team.ID)
}

func TestRepository_FindTeamByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	t.Run("finds existing team", func(t *testing.T) {
		found, err := repo.FindTeamByID(ctx, team.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, team.Name, found.Name)
	})

	t.Run("returns nil for non-existent team", func(t *testing.T) {
		found, err := repo.FindTeamByID(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_UpdateTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")
	team.Name = "Updated Team"

	err := repo.UpdateTeam(ctx, team)
	assert.NoError(t, err)

	found, err := repo.FindTeamByID(ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Team", found.Name)
}

func TestRepository_DeleteTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	// Add a member and invitation
	err := repo.AddUserToTeam(ctx, team.ID, user.ID, "member")
	require.NoError(t, err)

	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invite@example.com",
		Role:   "member",
	}
	err = repo.CreateTeamInvitation(ctx, invitation)
	require.NoError(t, err)

	// Set as current team
	err = repo.SetCurrentTeam(ctx, user.ID, team.ID)
	require.NoError(t, err)

	err = repo.DeleteTeam(ctx, team.ID)
	assert.NoError(t, err)

	found, err := repo.FindTeamByID(ctx, team.ID)
	assert.NoError(t, err)
	assert.Nil(t, found)

	// Verify user's current team is cleared
	foundUser, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, foundUser.CurrentTeamID)
}

func TestRepository_GetUserTeams(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team1 := createTestTeam(t, repo, user.ID, "Team 1")
	team2 := createTestTeam(t, repo, user.ID, "Team 2")

	// Add user to team2 as member
	err := repo.AddUserToTeam(ctx, team2.ID, user.ID, "member")
	require.NoError(t, err)

	teams, err := repo.GetUserTeams(ctx, user.ID)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)

	teamIDs := []string{teams[0].ID, teams[1].ID}
	assert.Contains(t, teamIDs, team1.ID)
	assert.Contains(t, teamIDs, team2.ID)
}

func TestRepository_GetTeamMembers(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	user2 := createTestUser(t, repo, "test2@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	err := repo.AddUserToTeam(ctx, team.ID, user.ID, "owner")
	require.NoError(t, err)

	err = repo.AddUserToTeam(ctx, team.ID, user2.ID, "member")
	require.NoError(t, err)

	members, err := repo.GetTeamMembers(ctx, team.ID)
	assert.NoError(t, err)
	assert.Len(t, members, 2)
}

// Team Member Repository Tests

func TestRepository_AddUserToTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	err := repo.AddUserToTeam(ctx, team.ID, user.ID, "member")
	assert.NoError(t, err)

	member, err := repo.GetTeamMember(ctx, team.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, member)
	assert.Equal(t, "member", member.Role)
}

func TestRepository_RemoveUserFromTeam(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	err := repo.AddUserToTeam(ctx, team.ID, user.ID, "member")
	require.NoError(t, err)

	err = repo.RemoveUserFromTeam(ctx, team.ID, user.ID)
	assert.NoError(t, err)

	member, err := repo.GetTeamMember(ctx, team.ID, user.ID)
	assert.NoError(t, err)
	assert.Nil(t, member)
}

func TestRepository_UpdateTeamMemberRole(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	err := repo.AddUserToTeam(ctx, team.ID, user.ID, "member")
	require.NoError(t, err)

	err = repo.UpdateTeamMemberRole(ctx, team.ID, user.ID, "admin")
	assert.NoError(t, err)

	member, err := repo.GetTeamMember(ctx, team.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "admin", member.Role)
}

func TestRepository_GetTeamMember(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	t.Run("returns nil when not a member", func(t *testing.T) {
		member, err := repo.GetTeamMember(ctx, team.ID, user.ID)
		assert.NoError(t, err)
		assert.Nil(t, member)
	})

	t.Run("returns member when exists", func(t *testing.T) {
		err := repo.AddUserToTeam(ctx, team.ID, user.ID, "member")
		require.NoError(t, err)

		member, err := repo.GetTeamMember(ctx, team.ID, user.ID)
		assert.NoError(t, err)
		assert.NotNil(t, member)
	})
}

func TestRepository_IsTeamMember(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	user2 := createTestUser(t, repo, "test2@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	t.Run("owner is member", func(t *testing.T) {
		isMember, err := repo.IsTeamMember(ctx, team.ID, user.ID)
		assert.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("non-member returns false", func(t *testing.T) {
		isMember, err := repo.IsTeamMember(ctx, team.ID, user2.ID)
		assert.NoError(t, err)
		assert.False(t, isMember)
	})

	t.Run("added member returns true", func(t *testing.T) {
		err := repo.AddUserToTeam(ctx, team.ID, user2.ID, "member")
		require.NoError(t, err)

		isMember, err := repo.IsTeamMember(ctx, team.ID, user2.ID)
		assert.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("returns false for non-existent team", func(t *testing.T) {
		isMember, err := repo.IsTeamMember(ctx, "nonexistent", user.ID)
		assert.NoError(t, err)
		assert.False(t, isMember)
	})
}

// Team Invitation Repository Tests

func TestRepository_CreateTeamInvitation(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invite@example.com",
		Role:   "member",
	}

	err := repo.CreateTeamInvitation(ctx, invitation)
	assert.NoError(t, err)
	assert.NotEmpty(t, invitation.ID)
}

func TestRepository_FindTeamInvitationByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invite@example.com",
		Role:   "member",
	}
	err := repo.CreateTeamInvitation(ctx, invitation)
	require.NoError(t, err)

	t.Run("finds existing invitation", func(t *testing.T) {
		found, err := repo.FindTeamInvitationByID(ctx, invitation.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, invitation.Email, found.Email)
	})

	t.Run("returns nil for non-existent invitation", func(t *testing.T) {
		found, err := repo.FindTeamInvitationByID(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_FindTeamInvitationByEmail(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invite@example.com",
		Role:   "member",
	}
	err := repo.CreateTeamInvitation(ctx, invitation)
	require.NoError(t, err)

	t.Run("finds existing invitation", func(t *testing.T) {
		found, err := repo.FindTeamInvitationByEmail(ctx, team.ID, "invite@example.com")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, invitation.ID, found.ID)
	})

	t.Run("returns nil for non-existent email", func(t *testing.T) {
		found, err := repo.FindTeamInvitationByEmail(ctx, team.ID, "nonexistent@example.com")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_GetTeamInvitations(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	for i := 0; i < 3; i++ {
		invitation := &models.TeamInvitation{
			TeamID: team.ID,
			Email:  "invite" + string(rune('1'+i)) + "@example.com",
			Role:   "member",
		}
		err := repo.CreateTeamInvitation(ctx, invitation)
		require.NoError(t, err)
	}

	invitations, err := repo.GetTeamInvitations(ctx, team.ID)
	assert.NoError(t, err)
	assert.Len(t, invitations, 3)
}

func TestRepository_DeleteTeamInvitation(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")
	team := createTestTeam(t, repo, user.ID, "Test Team")

	invitation := &models.TeamInvitation{
		TeamID: team.ID,
		Email:  "invite@example.com",
		Role:   "member",
	}
	err := repo.CreateTeamInvitation(ctx, invitation)
	require.NoError(t, err)

	err = repo.DeleteTeamInvitation(ctx, invitation.ID)
	assert.NoError(t, err)

	found, err := repo.FindTeamInvitationByID(ctx, invitation.ID)
	assert.NoError(t, err)
	assert.Nil(t, found)
}

// Password Reset Token Repository Tests

func TestRepository_CreatePasswordResetToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	token := &models.PasswordResetToken{
		Email:     "test@example.com",
		Token:     "hashedtoken",
		CreatedAt: time.Now(),
	}

	err := repo.CreatePasswordResetToken(ctx, token)
	assert.NoError(t, err)
}

func TestRepository_CreatePasswordResetToken_ReplacesExisting(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	token1 := &models.PasswordResetToken{
		Email:     "test@example.com",
		Token:     "token1",
		CreatedAt: time.Now(),
	}
	err := repo.CreatePasswordResetToken(ctx, token1)
	require.NoError(t, err)

	token2 := &models.PasswordResetToken{
		Email:     "test@example.com",
		Token:     "token2",
		CreatedAt: time.Now(),
	}
	err = repo.CreatePasswordResetToken(ctx, token2)
	assert.NoError(t, err)

	found, err := repo.FindPasswordResetToken(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, "token2", found.Token)
}

func TestRepository_FindPasswordResetToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	token := &models.PasswordResetToken{
		Email:     "test@example.com",
		Token:     "hashedtoken",
		CreatedAt: time.Now(),
	}
	err := repo.CreatePasswordResetToken(ctx, token)
	require.NoError(t, err)

	t.Run("finds existing token", func(t *testing.T) {
		found, err := repo.FindPasswordResetToken(ctx, "test@example.com")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, token.Token, found.Token)
	})

	t.Run("returns nil for non-existent email", func(t *testing.T) {
		found, err := repo.FindPasswordResetToken(ctx, "nonexistent@example.com")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_DeletePasswordResetToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	token := &models.PasswordResetToken{
		Email:     "test@example.com",
		Token:     "hashedtoken",
		CreatedAt: time.Now(),
	}
	err := repo.CreatePasswordResetToken(ctx, token)
	require.NoError(t, err)

	err = repo.DeletePasswordResetToken(ctx, "test@example.com")
	assert.NoError(t, err)

	found, err := repo.FindPasswordResetToken(ctx, "test@example.com")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

// Personal Access Token Repository Tests

func TestRepository_CreatePersonalAccessToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	token := &models.PersonalAccessToken{
		UserID: user.ID,
		Name:   "Test Token",
		Token:  "token123",
	}

	err := repo.CreatePersonalAccessToken(ctx, token)
	assert.NoError(t, err)
	assert.NotEmpty(t, token.ID)
}

func TestRepository_FindPersonalAccessToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	token := &models.PersonalAccessToken{
		UserID: user.ID,
		Name:   "Test Token",
		Token:  "token123",
	}
	err := repo.CreatePersonalAccessToken(ctx, token)
	require.NoError(t, err)

	t.Run("finds existing token", func(t *testing.T) {
		found, err := repo.FindPersonalAccessToken(ctx, "token123")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("returns nil for non-existent token", func(t *testing.T) {
		found, err := repo.FindPersonalAccessToken(ctx, "nonexistent")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_UpdatePersonalAccessTokenLastUsed(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	token := &models.PersonalAccessToken{
		UserID: user.ID,
		Name:   "Test Token",
		Token:  "token123",
	}
	err := repo.CreatePersonalAccessToken(ctx, token)
	require.NoError(t, err)

	assert.Nil(t, token.LastUsedAt)

	err = repo.UpdatePersonalAccessTokenLastUsed(ctx, token.ID)
	assert.NoError(t, err)

	found, err := repo.FindPersonalAccessToken(ctx, "token123")
	require.NoError(t, err)
	assert.NotNil(t, found.LastUsedAt)
}

func TestRepository_DeletePersonalAccessToken(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	token := &models.PersonalAccessToken{
		UserID: user.ID,
		Name:   "Test Token",
		Token:  "token123",
	}
	err := repo.CreatePersonalAccessToken(ctx, token)
	require.NoError(t, err)

	err = repo.DeletePersonalAccessToken(ctx, token.ID)
	assert.NoError(t, err)

	found, err := repo.FindPersonalAccessToken(ctx, "token123")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

func TestRepository_GetUserPersonalAccessTokens(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "test@example.com")

	for i := 0; i < 3; i++ {
		token := &models.PersonalAccessToken{
			UserID: user.ID,
			Name:   "Token " + string(rune('1'+i)),
			Token:  "token" + string(rune('1'+i)),
		}
		err := repo.CreatePersonalAccessToken(ctx, token)
		require.NoError(t, err)
	}

	tokens, err := repo.GetUserPersonalAccessTokens(ctx, user.ID)
	assert.NoError(t, err)
	assert.Len(t, tokens, 3)
}

// Transaction Tests

func TestRepository_Transaction(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("commits on success", func(t *testing.T) {
		err := repo.Transaction(ctx, func(tx *repositories.Repository) error {
			user := &models.User{
				Name:     "Transaction User",
				Email:    "tx@example.com",
				Password: "password",
			}
			return tx.CreateUser(ctx, user)
		})
		assert.NoError(t, err)

		found, err := repo.FindUserByEmail(ctx, "tx@example.com")
		require.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("rolls back on error", func(t *testing.T) {
		err := repo.Transaction(ctx, func(tx *repositories.Repository) error {
			user := &models.User{
				Name:     "Rollback User",
				Email:    "rollback@example.com",
				Password: "password",
			}
			if err := tx.CreateUser(ctx, user); err != nil {
				return err
			}

			return assert.AnError
		})
		assert.Error(t, err)

		found, err := repo.FindUserByEmail(ctx, "rollback@example.com")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestRepository_DB(t *testing.T) {
	repo, db := setupTestRepository(t)
	assert.Equal(t, db, repo.DB())
}
