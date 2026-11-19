package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts/mocks"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func passkeyLoginFixture(t *testing.T) (*PasskeyService, *miniredis.Miniredis, *models.User) {
	t.Helper()
	redis := miniredis.RunT(t)
	sessionCache := cache.NewRedisCache(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, sessionCache.Close()) })

	user := &models.User{
		BaseModel: basemodels.BaseModel{ID: "passkey-user"},
		Email:     "passkey@example.com",
	}
	registry := mocks.NewRepositoryRegistry(t)
	userRepo := mocks.NewUserRepository(t)
	passkeyRepo := mocks.NewPasskeyRepository(t)
	registry.EXPECT().User().Return(userRepo).Maybe()
	registry.EXPECT().Passkey().Return(passkeyRepo).Maybe()
	userRepo.EXPECT().FindByEmail(mock.Anything, user.Email).Return(user, nil).Maybe()
	passkeyRepo.EXPECT().FindByUserID(mock.Anything, user.ID).Return([]models.Passkey{
		{UserID: user.ID, CredentialID: "Y3JlZGVudGlhbA"},
	}, nil).Maybe()

	cfg := &config.Config{}
	cfg.Passkey = config.PasskeyConfig{
		RPName: "launchctl", RPID: "example.com", RPOrigin: "https://example.com", Timeout: 60000,
	}
	logger := zerolog.Nop()
	service, err := NewPasskeyService(registry, cfg, sessionCache, &logger)
	require.NoError(t, err)
	return service, redis, user
}

func passkeyAssertionForChallenge(challenge string) *protocol.ParsedCredentialAssertionData {
	return &protocol.ParsedCredentialAssertionData{
		Response: protocol.ParsedAssertionResponse{
			CollectedClientData: protocol.CollectedClientData{Challenge: challenge},
		},
	}
}

func TestPasskeyLoginSessionsDoNotInterfere(t *testing.T) {
	service, _, user := passkeyLoginFixture(t)
	ctx := context.Background()

	// An email-based attempt can remain pending when another tab uses autofill.
	emailOptions, err := service.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	discoverableOptions, err := service.BeginLogin(ctx, "")
	require.NoError(t, err)
	require.Empty(t, discoverableOptions.Response.AllowedCredentials)

	discoverableChallenge := discoverableOptions.Response.Challenge.String()
	session, err := service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(discoverableChallenge))
	require.NoError(t, err)
	require.Equal(t, discoverableChallenge, session.Challenge)
	require.Empty(t, session.UserID)

	emailChallenge := emailOptions.Response.Challenge.String()
	session, err = service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(emailChallenge))
	require.NoError(t, err)
	require.Equal(t, emailChallenge, session.Challenge)
	require.Equal(t, []byte(user.ID), session.UserID)
}

func TestPasskeyEmailLoginSessionsCanCompleteIndependently(t *testing.T) {
	service, _, user := passkeyLoginFixture(t)
	ctx := context.Background()
	first, err := service.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	second, err := service.BeginLogin(ctx, user.Email)
	require.NoError(t, err)

	for _, options := range []*protocol.CredentialAssertion{first, second} {
		challenge := options.Response.Challenge.String()
		session, err := service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(challenge))
		require.NoError(t, err)
		require.Equal(t, challenge, session.Challenge)

		_, err = service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(challenge))
		require.Error(t, err, "a completed ceremony must not be reused")
	}
}

func TestPasskeyUnknownChallengeDoesNotConsumePendingLogin(t *testing.T) {
	service, _, user := passkeyLoginFixture(t)
	ctx := context.Background()
	options, err := service.BeginLogin(ctx, user.Email)
	require.NoError(t, err)

	_, err = service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge("unknown-challenge"))
	require.Error(t, err)

	challenge := options.Response.Challenge.String()
	_, err = service.findLoginSession(ctx, "different-user", passkeyAssertionForChallenge(challenge))
	require.Error(t, err)

	session, err := service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(challenge))
	require.NoError(t, err)
	require.Equal(t, challenge, session.Challenge)
}

func TestPasskeyLoginSessionExpires(t *testing.T) {
	for _, email := range []string{"", "passkey@example.com"} {
		t.Run(email, func(t *testing.T) {
			service, redis, user := passkeyLoginFixture(t)
			ctx := context.Background()
			options, err := service.BeginLogin(ctx, email)
			require.NoError(t, err)
			redis.FastForward(5 * time.Minute)

			_, err = service.findLoginSession(ctx, user.ID, passkeyAssertionForChallenge(options.Response.Challenge.String()))
			require.Error(t, err)
		})
	}
}
