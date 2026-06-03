package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// fakePlatformInviteReader records calls so tests can assert what the register
// flow threaded through, and lets each test control the lookup outcome.
type fakePlatformInviteReader struct {
	email       string
	trialEndsAt time.Time
	ok          bool
	lookupErr   error

	lookupToken string
	lookupCount int

	acceptToken  string
	acceptTeamID string
	acceptCount  int
	acceptErr    error
}

func (f *fakePlatformInviteReader) PlatformInviteByToken(_ context.Context, token string) (string, time.Time, bool, error) {
	f.lookupCount++
	f.lookupToken = token
	return f.email, f.trialEndsAt, f.ok, f.lookupErr
}

func (f *fakePlatformInviteReader) AcceptPlatformInviteWithTrial(_ context.Context, token, teamID string) error {
	f.acceptCount++
	f.acceptToken = token
	f.acceptTeamID = teamID
	return f.acceptErr
}

func newServiceWithReader(reader PlatformInviteReader) *AuthService {
	logger := zerolog.Nop()
	svc := NewAuthService(nil, nil, &logger, nil)
	svc.SetPlatformInviteReader(reader)
	return svc
}

func userWithTeam(email, teamID string) *models.User {
	return &models.User{
		BaseModel:     basemodels.BaseModel{ID: "user_1"},
		Email:         email,
		CurrentTeamID: &teamID,
	}
}

func TestConsumePlatformInvite_GrantsTrialOnValidEmailMatch(t *testing.T) {
	reader := &fakePlatformInviteReader{
		email:       "Invitee@Example.com",
		trialEndsAt: time.Now().Add(24 * time.Hour),
		ok:          true,
	}
	svc := newServiceWithReader(reader)

	req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok-123"}
	user := userWithTeam("invitee@example.com", "team-1")

	svc.consumePlatformInvite(context.Background(), req, user)

	assert.Equal(t, 1, reader.lookupCount)
	assert.Equal(t, "tok-123", reader.lookupToken)
	assert.Equal(t, 1, reader.acceptCount)
	assert.Equal(t, "tok-123", reader.acceptToken)
	assert.Equal(t, "team-1", reader.acceptTeamID)
}

func TestConsumePlatformInvite_SkipsOnEmailMismatch(t *testing.T) {
	reader := &fakePlatformInviteReader{
		email: "someone-else@example.com",
		ok:    true,
	}
	svc := newServiceWithReader(reader)

	req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok-123"}
	user := userWithTeam("invitee@example.com", "team-1")

	svc.consumePlatformInvite(context.Background(), req, user)

	assert.Equal(t, 1, reader.lookupCount)
	assert.Equal(t, 0, reader.acceptCount, "trial must not be granted on email mismatch")
}

func TestConsumePlatformInvite_SkipsOnLookupErrorOrNotOk(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		reader := &fakePlatformInviteReader{lookupErr: errors.New("db down")}
		svc := newServiceWithReader(reader)

		req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok"}
		svc.consumePlatformInvite(context.Background(), req, userWithTeam("invitee@example.com", "team-1"))

		assert.Equal(t, 0, reader.acceptCount, "lookup error must never grant a trial or block signup")
	})

	t.Run("not ok", func(t *testing.T) {
		reader := &fakePlatformInviteReader{ok: false}
		svc := newServiceWithReader(reader)

		req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok"}
		svc.consumePlatformInvite(context.Background(), req, userWithTeam("invitee@example.com", "team-1"))

		assert.Equal(t, 0, reader.acceptCount, "unusable invite must not grant a trial")
	})
}

func TestConsumePlatformInvite_SkipsWhenNoTokenOrNoTeam(t *testing.T) {
	t.Run("empty token", func(t *testing.T) {
		reader := &fakePlatformInviteReader{ok: true}
		svc := newServiceWithReader(reader)

		req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: ""}
		svc.consumePlatformInvite(context.Background(), req, userWithTeam("invitee@example.com", "team-1"))

		assert.Equal(t, 0, reader.lookupCount)
	})

	t.Run("no personal team", func(t *testing.T) {
		reader := &fakePlatformInviteReader{ok: true}
		svc := newServiceWithReader(reader)

		req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok"}
		user := &models.User{BaseModel: basemodels.BaseModel{ID: "user_1"}, Email: "invitee@example.com"}
		svc.consumePlatformInvite(context.Background(), req, user)

		assert.Equal(t, 0, reader.lookupCount)
	})

	t.Run("nil reader", func(t *testing.T) {
		logger := zerolog.Nop()
		svc := NewAuthService(nil, nil, &logger, nil)

		req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok"}
		// Must not panic when no reader is wired.
		svc.consumePlatformInvite(context.Background(), req, userWithTeam("invitee@example.com", "team-1"))
	})
}

func TestConsumePlatformInvite_AcceptErrorDoesNotPanic(t *testing.T) {
	reader := &fakePlatformInviteReader{
		email:     "invitee@example.com",
		ok:        true,
		acceptErr: errors.New("insert failed"),
	}
	svc := newServiceWithReader(reader)

	req := &dto.RegisterRequest{Email: "invitee@example.com", PlatformInviteToken: "tok"}
	// An accept error is logged, not propagated — registration still succeeds.
	svc.consumePlatformInvite(context.Background(), req, userWithTeam("invitee@example.com", "team-1"))

	assert.Equal(t, 1, reader.acceptCount)
}

func TestPlatformInviteEmailMatches(t *testing.T) {
	assert.True(t, platformInviteEmailMatches("a@b.com", "a@b.com"))
	assert.True(t, platformInviteEmailMatches("A@B.com", "a@b.COM"))
	assert.False(t, platformInviteEmailMatches("a@b.com", "c@d.com"))
	assert.False(t, platformInviteEmailMatches("", "a@b.com"))
}
