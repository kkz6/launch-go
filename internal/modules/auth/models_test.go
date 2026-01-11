package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&User{}, &Team{}, &TeamMember{}, &TeamInvitation{}, &PersonalAccessToken{}, &PasswordResetToken{})
	require.NoError(t, err)

	return db
}

// User Model Tests

func TestUser_TableName(t *testing.T) {
	user := &User{}
	assert.Equal(t, "users", user.TableName())
}

func TestUser_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("generates ULID if empty", func(t *testing.T) {
		user := &User{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "password",
		}

		err := db.Create(user).Error
		require.NoError(t, err)
		assert.NotEmpty(t, user.ID)
		assert.Len(t, user.ID, 26)
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		customID := "01HQWXYZ1234567890ABCDEF"
		user := &User{
			ID:       customID,
			Name:     "Test User 2",
			Email:    "test2@example.com",
			Password: "password",
		}

		err := db.Create(user).Error
		require.NoError(t, err)
		assert.Equal(t, customID, user.ID)
	})
}

func TestUser_HasVerifiedEmail(t *testing.T) {
	t.Run("returns false when nil", func(t *testing.T) {
		user := &User{}
		assert.False(t, user.HasVerifiedEmail())
	})

	t.Run("returns true when set", func(t *testing.T) {
		now := time.Now()
		user := &User{EmailVerifiedAt: &now}
		assert.True(t, user.HasVerifiedEmail())
	})
}

func TestUser_HasEnabledTwoFactorAuthentication(t *testing.T) {
	t.Run("returns false when secret is nil", func(t *testing.T) {
		user := &User{}
		assert.False(t, user.HasEnabledTwoFactorAuthentication())
	})

	t.Run("returns false when confirmed at is nil", func(t *testing.T) {
		secret := "secret"
		user := &User{TwoFactorSecret: &secret}
		assert.False(t, user.HasEnabledTwoFactorAuthentication())
	})

	t.Run("returns true when both set", func(t *testing.T) {
		secret := "secret"
		now := time.Now()
		user := &User{
			TwoFactorSecret:      &secret,
			TwoFactorConfirmedAt: &now,
		}
		assert.True(t, user.HasEnabledTwoFactorAuthentication())
	})
}

func TestUser_TwoFactorEnabled(t *testing.T) {
	secret := "secret"
	now := time.Now()

	user := &User{
		TwoFactorSecret:      &secret,
		TwoFactorConfirmedAt: &now,
	}
	assert.True(t, user.TwoFactorEnabled())
}

func TestUser_ProfilePhotoURL(t *testing.T) {
	t.Run("returns custom photo when set", func(t *testing.T) {
		path := "https://example.com/photo.jpg"
		user := &User{ProfilePhotoPath: &path}
		assert.Equal(t, path, user.ProfilePhotoURL())
	})

	t.Run("returns default when empty", func(t *testing.T) {
		user := &User{Name: "Test User"}
		url := user.ProfilePhotoURL()
		assert.Contains(t, url, "ui-avatars.com")
	})

	t.Run("returns default when path is empty string", func(t *testing.T) {
		empty := ""
		user := &User{Name: "Test User", ProfilePhotoPath: &empty}
		url := user.ProfilePhotoURL()
		assert.Contains(t, url, "ui-avatars.com")
	})
}

func TestUser_DefaultProfilePhotoURL(t *testing.T) {
	t.Run("uses name for avatar", func(t *testing.T) {
		user := &User{Name: "Test User"}
		url := user.DefaultProfilePhotoURL()
		assert.Contains(t, url, "Test User")
	})

	t.Run("uses U for empty name", func(t *testing.T) {
		user := &User{}
		url := user.DefaultProfilePhotoURL()
		assert.Contains(t, url, "U")
	})
}

func TestUser_OwnsTeam(t *testing.T) {
	user := &User{ID: "user1"}
	team := &Team{OwnerID: "user1"}

	assert.True(t, user.OwnsTeam(team))

	team2 := &Team{OwnerID: "user2"}
	assert.False(t, user.OwnsTeam(team2))
}

func TestUser_BelongsToTeam(t *testing.T) {
	t.Run("returns true if owner", func(t *testing.T) {
		user := &User{ID: "user1"}
		team := &Team{ID: "team1", OwnerID: "user1"}
		assert.True(t, user.BelongsToTeam(team))
	})

	t.Run("returns true if member", func(t *testing.T) {
		user := &User{
			ID:    "user1",
			Teams: []Team{{ID: "team1"}},
		}
		team := &Team{ID: "team1", OwnerID: "user2"}
		assert.True(t, user.BelongsToTeam(team))
	})

	t.Run("returns false if not member", func(t *testing.T) {
		user := &User{
			ID:    "user1",
			Teams: []Team{{ID: "team2"}},
		}
		team := &Team{ID: "team1", OwnerID: "user2"}
		assert.False(t, user.BelongsToTeam(team))
	})
}

// Team Model Tests

func TestTeam_TableName(t *testing.T) {
	team := &Team{}
	assert.Equal(t, "teams", team.TableName())
}

func TestTeam_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	t.Run("generates ULID if empty", func(t *testing.T) {
		team := &Team{
			Name:    "Test Team",
			OwnerID: "owner1",
		}

		err := db.Create(team).Error
		require.NoError(t, err)
		assert.NotEmpty(t, team.ID)
		assert.Len(t, team.ID, 26)
	})
}

func TestTeam_ImageURL(t *testing.T) {
	t.Run("returns custom image when set", func(t *testing.T) {
		path := "https://example.com/team.jpg"
		team := &Team{ImagePath: &path}
		assert.Equal(t, path, team.ImageURL())
	})

	t.Run("returns default when empty", func(t *testing.T) {
		team := &Team{Name: "Test Team"}
		url := team.ImageURL()
		assert.Contains(t, url, "ui-avatars.com")
	})
}

func TestTeam_DefaultImageURL(t *testing.T) {
	t.Run("uses name for avatar", func(t *testing.T) {
		team := &Team{Name: "Test Team"}
		url := team.DefaultImageURL()
		assert.Contains(t, url, "Test Team")
	})

	t.Run("uses T for empty name", func(t *testing.T) {
		team := &Team{}
		url := team.DefaultImageURL()
		assert.Contains(t, url, "T")
	})
}

func TestTeam_HasUser(t *testing.T) {
	t.Run("returns true for owner", func(t *testing.T) {
		team := &Team{OwnerID: "user1"}
		user := &User{ID: "user1"}
		assert.True(t, team.HasUser(user))
	})

	t.Run("returns true for member", func(t *testing.T) {
		team := &Team{
			OwnerID: "user2",
			Members: []User{{ID: "user1"}},
		}
		user := &User{ID: "user1"}
		assert.True(t, team.HasUser(user))
	})

	t.Run("returns false for non-member", func(t *testing.T) {
		team := &Team{
			OwnerID: "user2",
			Members: []User{{ID: "user3"}},
		}
		user := &User{ID: "user1"}
		assert.False(t, team.HasUser(user))
	})
}

// TeamMember Model Tests

func TestTeamMember_TableName(t *testing.T) {
	tm := &TeamMember{}
	assert.Equal(t, "team_members", tm.TableName())
}

func TestTeamMember_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a user and team
	user := &User{Name: "Test", Email: "test@test.com", Password: "pass"}
	require.NoError(t, db.Create(user).Error)

	team := &Team{Name: "Test Team", OwnerID: user.ID}
	require.NoError(t, db.Create(team).Error)

	t.Run("generates ULID if empty", func(t *testing.T) {
		tm := &TeamMember{
			TeamID: team.ID,
			UserID: user.ID,
			Role:   "member",
		}

		err := db.Create(tm).Error
		require.NoError(t, err)
		assert.NotEmpty(t, tm.ID)
		assert.Len(t, tm.ID, 26)
	})
}

// TeamInvitation Model Tests

func TestTeamInvitation_TableName(t *testing.T) {
	ti := &TeamInvitation{}
	assert.Equal(t, "team_invitations", ti.TableName())
}

func TestTeamInvitation_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a team
	user := &User{Name: "Test", Email: "test@test.com", Password: "pass"}
	require.NoError(t, db.Create(user).Error)

	team := &Team{Name: "Test Team", OwnerID: user.ID}
	require.NoError(t, db.Create(team).Error)

	t.Run("generates ULID if empty", func(t *testing.T) {
		ti := &TeamInvitation{
			TeamID: team.ID,
			Email:  "invite@example.com",
			Role:   "member",
		}

		err := db.Create(ti).Error
		require.NoError(t, err)
		assert.NotEmpty(t, ti.ID)
		assert.Len(t, ti.ID, 26)
	})
}

// PersonalAccessToken Model Tests

func TestPersonalAccessToken_TableName(t *testing.T) {
	pat := &PersonalAccessToken{}
	assert.Equal(t, "personal_access_tokens", pat.TableName())
}

func TestPersonalAccessToken_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	user := &User{Name: "Test", Email: "test@test.com", Password: "pass"}
	require.NoError(t, db.Create(user).Error)

	t.Run("generates ULID if empty", func(t *testing.T) {
		pat := &PersonalAccessToken{
			UserID: user.ID,
			Name:   "Test Token",
			Token:  "token123",
		}

		err := db.Create(pat).Error
		require.NoError(t, err)
		assert.NotEmpty(t, pat.ID)
		assert.Len(t, pat.ID, 26)
	})
}

func TestPersonalAccessToken_IsExpired(t *testing.T) {
	t.Run("returns false when no expiry", func(t *testing.T) {
		pat := &PersonalAccessToken{}
		assert.False(t, pat.IsExpired())
	})

	t.Run("returns false when not expired", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		pat := &PersonalAccessToken{ExpiresAt: &future}
		assert.False(t, pat.IsExpired())
	})

	t.Run("returns true when expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		pat := &PersonalAccessToken{ExpiresAt: &past}
		assert.True(t, pat.IsExpired())
	})
}

// PasswordResetToken Model Tests

func TestPasswordResetToken_TableName(t *testing.T) {
	prt := &PasswordResetToken{}
	assert.Equal(t, "password_reset_tokens", prt.TableName())
}

func TestPasswordResetToken_IsExpired(t *testing.T) {
	t.Run("returns false when not expired", func(t *testing.T) {
		prt := &PasswordResetToken{CreatedAt: time.Now()}
		assert.False(t, prt.IsExpired())
	})

	t.Run("returns true when expired", func(t *testing.T) {
		prt := &PasswordResetToken{CreatedAt: time.Now().Add(-2 * time.Hour)}
		assert.True(t, prt.IsExpired())
	})

	t.Run("returns false at 59 minutes", func(t *testing.T) {
		prt := &PasswordResetToken{CreatedAt: time.Now().Add(-59 * time.Minute)}
		assert.False(t, prt.IsExpired())
	})

	t.Run("returns true at 61 minutes", func(t *testing.T) {
		prt := &PasswordResetToken{CreatedAt: time.Now().Add(-61 * time.Minute)}
		assert.True(t, prt.IsExpired())
	})
}

// GenerateToken Tests

func TestGenerateToken(t *testing.T) {
	t.Run("generates token of correct length", func(t *testing.T) {
		token, err := GenerateToken(32)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err := GenerateToken(32)
		require.NoError(t, err)

		token2, err := GenerateToken(32)
		require.NoError(t, err)

		assert.NotEqual(t, token1, token2)
	})
}

// TeamRole Tests

func TestTeamRole_IsValid(t *testing.T) {
	t.Run("owner is valid", func(t *testing.T) {
		assert.True(t, TeamRoleOwner.IsValid())
	})

	t.Run("admin is valid", func(t *testing.T) {
		assert.True(t, TeamRoleAdmin.IsValid())
	})

	t.Run("member is valid", func(t *testing.T) {
		assert.True(t, TeamRoleMember.IsValid())
	})

	t.Run("invalid role", func(t *testing.T) {
		role := TeamRole("invalid")
		assert.False(t, role.IsValid())
	})
}

func TestTeamRole_String(t *testing.T) {
	assert.Equal(t, "owner", TeamRoleOwner.String())
	assert.Equal(t, "admin", TeamRoleAdmin.String())
	assert.Equal(t, "member", TeamRoleMember.String())
}
