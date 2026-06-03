package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// fakeEmailSender records the last Send call so tests can assert delivery.
type fakeEmailSender struct {
	calls      int
	lastTo     string
	lastBody   string
	shouldFail bool
}

func (f *fakeEmailSender) Send(_ context.Context, to, _ /*subject*/, body string, _ bool) error {
	f.calls++
	f.lastTo = to
	f.lastBody = body
	if f.shouldFail {
		return assertErr
	}
	return nil
}

var assertErr = &fakeError{"send failed"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }

// setupInvitationService returns a staff service backed by an in-memory sqlite
// DB holding the users + platform_invitations tables, with a fake email sender
// and a fixed frontend URL wired in.
func setupInvitationService(t *testing.T) (*Service, *gorm.DB, *fakeEmailSender) {
	t.Helper()

	// Email templates read the global config; initialise it so Build() works.
	templates.Initialize("Launch", "https://app.example.com")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&authmodels.User{}, &authmodels.PlatformInvitation{}))

	sender := &fakeEmailSender{}
	svc := NewService(repositories.NewRegistry(db))
	svc.SetInvitationDeps(sender, "https://app.example.com")

	return svc, db, sender
}

func TestInviteUser_CreatesRowAndSendsEmail(t *testing.T) {
	svc, db, sender := setupInvitationService(t)
	ctx := context.Background()

	trialEndsAt := time.Now().Add(30 * 24 * time.Hour)
	before := time.Now()

	invitation, err := svc.InviteUser(ctx, "invitee@example.com", trialEndsAt, "inviter-id")
	require.NoError(t, err)
	require.NotNil(t, invitation)

	assert.Equal(t, "invitee@example.com", invitation.Email)
	assert.Equal(t, "inviter-id", invitation.InvitedBy)
	assert.NotEmpty(t, invitation.ID)
	assert.NotEmpty(t, invitation.Token)
	assert.GreaterOrEqual(t, len(invitation.Token), 32)

	// ExpiresAt ~ now + 14d.
	expectedExpiry := before.Add(invitationValidity)
	assert.WithinDuration(t, expectedExpiry, invitation.ExpiresAt, time.Minute)

	// Row is queryable.
	var stored authmodels.PlatformInvitation
	require.NoError(t, db.First(&stored, "email = ?", "invitee@example.com").Error)
	assert.Equal(t, invitation.Token, stored.Token)

	// Email was sent to the invitee with a body carrying the invite URL + token.
	assert.Equal(t, 1, sender.calls)
	assert.Equal(t, "invitee@example.com", sender.lastTo)
	assert.True(t, strings.Contains(sender.lastBody, "/register?invite="+invitation.Token),
		"email body should contain the invite URL with the token")
}

func TestInviteUser_GeneratesUniqueTokens(t *testing.T) {
	svc, _, _ := setupInvitationService(t)
	ctx := context.Background()
	trialEndsAt := time.Now().Add(30 * 24 * time.Hour)

	first, err := svc.InviteUser(ctx, "a@example.com", trialEndsAt, "inviter")
	require.NoError(t, err)
	second, err := svc.InviteUser(ctx, "b@example.com", trialEndsAt, "inviter")
	require.NoError(t, err)

	assert.NotEqual(t, first.Token, second.Token)
}

func TestInviteUser_UserAlreadyExists(t *testing.T) {
	svc, db, sender := setupInvitationService(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&authmodels.User{
		Name:  "Existing",
		Email: "taken@example.com",
	}).Error)

	invitation, err := svc.InviteUser(ctx, "taken@example.com", time.Now().Add(24*time.Hour), "inviter")
	require.ErrorIs(t, err, ErrUserAlreadyExists)
	assert.Nil(t, invitation)

	// No invite row, no email.
	var count int64
	require.NoError(t, db.Model(&authmodels.PlatformInvitation{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	assert.Equal(t, 0, sender.calls)
}

func TestInviteUser_PendingInviteExists(t *testing.T) {
	svc, _, sender := setupInvitationService(t)
	ctx := context.Background()
	trialEndsAt := time.Now().Add(30 * 24 * time.Hour)

	_, err := svc.InviteUser(ctx, "dupe@example.com", trialEndsAt, "inviter")
	require.NoError(t, err)

	sender.calls = 0
	invitation, err := svc.InviteUser(ctx, "dupe@example.com", trialEndsAt, "inviter")
	require.ErrorIs(t, err, ErrInvitePending)
	assert.Nil(t, invitation)
	assert.Equal(t, 0, sender.calls)
}

func TestInviteUser_TrialDateInPast(t *testing.T) {
	svc, db, sender := setupInvitationService(t)
	ctx := context.Background()

	invitation, err := svc.InviteUser(ctx, "past@example.com", time.Now().Add(-time.Hour), "inviter")
	require.ErrorIs(t, err, ErrInvalidTrialDate)
	assert.Nil(t, invitation)

	var count int64
	require.NoError(t, db.Model(&authmodels.PlatformInvitation{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
	assert.Equal(t, 0, sender.calls)
}

func TestInviteUser_EmailRequired(t *testing.T) {
	svc, _, _ := setupInvitationService(t)

	invitation, err := svc.InviteUser(context.Background(), "  ", time.Now().Add(time.Hour), "inviter")
	require.ErrorIs(t, err, ErrInvalidInviteEmail)
	assert.Nil(t, invitation)
}

func TestInviteUser_EmailFailureKeepsRow(t *testing.T) {
	svc, db, sender := setupInvitationService(t)
	sender.shouldFail = true
	ctx := context.Background()

	invitation, err := svc.InviteUser(ctx, "kept@example.com", time.Now().Add(24*time.Hour), "inviter")
	// The row is still returned even though delivery failed.
	require.Error(t, err)
	require.NotNil(t, invitation)

	var count int64
	require.NoError(t, db.Model(&authmodels.PlatformInvitation{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestListPendingInvitations_OnlyPending(t *testing.T) {
	svc, db, _ := setupInvitationService(t)
	ctx := context.Background()
	now := time.Now()

	accepted := now.Add(-time.Hour)
	require.NoError(t, db.Create(&authmodels.PlatformInvitation{
		ID: "inv-accepted", Email: "accepted@example.com", Token: "tok-accepted",
		TrialEndsAt: now.Add(24 * time.Hour), InvitedBy: "inviter",
		AcceptedAt: &accepted, ExpiresAt: now.Add(14 * 24 * time.Hour),
	}).Error)
	require.NoError(t, db.Create(&authmodels.PlatformInvitation{
		ID: "inv-expired", Email: "expired@example.com", Token: "tok-expired",
		TrialEndsAt: now.Add(24 * time.Hour), InvitedBy: "inviter",
		ExpiresAt: now.Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&authmodels.PlatformInvitation{
		ID: "inv-pending", Email: "pending@example.com", Token: "tok-pending",
		TrialEndsAt: now.Add(24 * time.Hour), InvitedBy: "inviter",
		ExpiresAt: now.Add(14 * 24 * time.Hour),
	}).Error)

	invitations, total, err := svc.ListPendingInvitations(ctx, 25, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, invitations, 1)
	assert.Equal(t, "pending@example.com", invitations[0].Email)
}

func TestRevokeInvitation(t *testing.T) {
	svc, db, _ := setupInvitationService(t)
	ctx := context.Background()
	trialEndsAt := time.Now().Add(30 * 24 * time.Hour)

	invitation, err := svc.InviteUser(ctx, "revoke@example.com", trialEndsAt, "inviter")
	require.NoError(t, err)

	require.NoError(t, svc.RevokeInvitation(ctx, invitation.ID))

	var count int64
	require.NoError(t, db.Model(&authmodels.PlatformInvitation{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)

	// Revoking a missing invite reports not found.
	err = svc.RevokeInvitation(ctx, "does-not-exist")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
