package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

const (
	passkeySessionTTL       = 5 * time.Minute
	passkeyRegKeyPrefix     = "passkey:reg:"
	passkeyAuthKeyPrefix    = "passkey:auth:"
	passkeyAuthDiscoverable = "discoverable"
)

// PasskeyService handles passkey management and WebAuthn ceremonies
type PasskeyService struct {
	repos    contracts.RepositoryRegistry
	config   *config.Config
	cache    cache.Cache
	webauthn *webauthn.WebAuthn
	logger   *zerolog.Logger
}

// NewPasskeyService creates a new PasskeyService instance
func NewPasskeyService(repos contracts.RepositoryRegistry, cfg *config.Config, c cache.Cache, logger *zerolog.Logger) (*PasskeyService, error) {
	rpName := cfg.Passkey.RPName
	if rpName == "" {
		rpName = cfg.App.Name
	}

	rpID := cfg.Passkey.RPID
	if rpID == "" {
		if parsed, err := url.Parse(cfg.App.URL); err == nil {
			rpID = parsed.Hostname()
		}
	}

	rpOrigin := cfg.Passkey.RPOrigin
	if rpOrigin == "" {
		rpOrigin = cfg.App.URL
	}

	timeout := time.Duration(cfg.Passkey.Timeout) * time.Millisecond

	wauthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: rpName,
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Timeout:    timeout,
				TimeoutUVD: timeout,
			},
			Registration: webauthn.TimeoutConfig{
				Timeout:    timeout,
				TimeoutUVD: timeout,
			},
		},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementPreferred,
			UserVerification:   protocol.VerificationPreferred,
			RequireResidentKey: protocol.ResidentKeyNotRequired(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize webauthn: %w", err)
	}

	return &PasskeyService{
		repos:    repos,
		config:   cfg,
		cache:    c,
		webauthn: wauthn,
		logger:   logger,
	}, nil
}

// BeginRegistration generates WebAuthn registration options
func (s *PasskeyService) BeginRegistration(ctx context.Context, userID string) (*protocol.CredentialCreation, error) {
	user, passkeys, err := s.getUserAndPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(passkeys) >= s.config.Passkey.MaxPerUser {
		return nil, fiberutil.BadRequest("Maximum number of passkeys reached")
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	var excludeList []protocol.CredentialDescriptor
	for _, cred := range webAuthnUser.WebAuthnCredentials() {
		excludeList = append(excludeList, protocol.CredentialDescriptor{
			Type:            protocol.PublicKeyCredentialType,
			CredentialID:    cred.ID,
			Transport:       cred.Transport,
			AttestationType: cred.AttestationType,
		})
	}

	creation, session, err := s.webauthn.BeginRegistration(
		webAuthnUser,
		webauthn.WithExclusions(excludeList),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to begin registration")
	}

	if err := s.storeSession(ctx, passkeyRegKeyPrefix+userID, session); err != nil {
		return nil, fiberutil.BadRequest("Failed to start registration session")
	}

	return creation, nil
}

// FinishRegistration completes WebAuthn registration
func (s *PasskeyService) FinishRegistration(ctx context.Context, userID string, body []byte, name *string) (*models.Passkey, error) {
	user, passkeys, err := s.getUserAndPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	session, err := s.loadSession(ctx, passkeyRegKeyPrefix+userID)
	if err != nil {
		return nil, fiberutil.BadRequest("Registration session expired or not found")
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to parse registration response")
	}

	credential, err := s.webauthn.CreateCredential(webAuthnUser, *session, parsedResponse)
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to verify registration")
	}

	passkey := models.PasskeyFromCredential(userID, credential, name)
	if err := s.repos.Passkey().Create(ctx, passkey); err != nil {
		return nil, fmt.Errorf("failed to save passkey: %w", err)
	}

	return passkey, nil
}

// BeginLogin generates WebAuthn authentication options
func (s *PasskeyService) BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, error) {
	if email == "" {
		return s.beginDiscoverableLogin(ctx)
	}

	return s.beginEmailLogin(ctx, email)
}

// FinishLogin completes WebAuthn authentication and returns the authenticated user
func (s *PasskeyService) FinishLogin(ctx context.Context, body []byte) (*models.User, error) {
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fiberutil.BadRequest("Invalid passkey response")
	}

	// Look up the passkey by credential ID to get the real user ID.
	credentialID := base64.RawURLEncoding.EncodeToString(parsedResponse.RawID)
	passkey, err := s.repos.Passkey().FindByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, fiberutil.Unauthorized("Passkey not recognized")
	}

	session, err := s.findLoginSession(ctx, passkey.UserID, parsedResponse)
	if err != nil {
		return nil, err
	}

	user, passkeys, err := s.getUserAndPasskeys(ctx, passkey.UserID)
	if err != nil {
		return nil, fiberutil.Unauthorized("User not found")
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	credential, err := s.verifyLogin(webAuthnUser, session, parsedResponse)
	if err != nil {
		return nil, err
	}

	s.updatePasskeyUsage(ctx, parsedResponse.RawID, credential)

	return user, nil
}

// GetUserPasskeys returns all passkeys for a user
func (s *PasskeyService) GetUserPasskeys(ctx context.Context, userID string) ([]models.Passkey, error) {
	return s.repos.Passkey().FindByUserID(ctx, userID)
}

// UpdatePasskeyName updates a passkey's name
func (s *PasskeyService) UpdatePasskeyName(ctx context.Context, passkeyID, userID, name string) error {
	passkey, err := s.repos.Passkey().FindByID(ctx, passkeyID)
	if err != nil {
		return err
	}

	if passkey == nil || passkey.UserID != userID {
		return fiberutil.NotFound()
	}

	passkey.Name = &name

	return s.repos.Passkey().Update(ctx, passkey)
}

// DeletePasskey deletes a passkey for a user
func (s *PasskeyService) DeletePasskey(ctx context.Context, passkeyID, userID string) error {
	return s.repos.Passkey().DeleteByUserID(ctx, passkeyID, userID)
}

// --- private helpers ---

func (s *PasskeyService) beginEmailLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, error) {
	user, err := s.repos.User().FindByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to retrieve passkeys")
	}

	if len(passkeys) == 0 {
		return nil, fiberutil.BadRequest("No passkeys registered for this account")
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)
	assertion, session, err := s.webauthn.BeginLogin(webAuthnUser)
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to initialize passkey authentication")
	}

	if err := s.storeSession(ctx, passkeyAuthKeyPrefix+user.ID, session); err != nil {
		return nil, fiberutil.BadRequest("Failed to start passkey authentication session")
	}

	return assertion, nil
}

func (s *PasskeyService) beginDiscoverableLogin(ctx context.Context) (*protocol.CredentialAssertion, error) {
	assertion, session, err := s.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, fiberutil.BadRequest("Failed to initialize passkey authentication")
	}

	cacheKey := passkeyAuthKeyPrefix + passkeyAuthDiscoverable + ":" + session.Challenge
	if err := s.storeSession(ctx, cacheKey, session); err != nil {
		return nil, fiberutil.BadRequest("Failed to start passkey authentication session")
	}

	return assertion, nil
}

func (s *PasskeyService) findLoginSession(ctx context.Context, userID string, parsedResponse *protocol.ParsedCredentialAssertionData) (*webauthn.SessionData, error) {
	// Try user-specific session first, then discoverable session
	session, err := s.loadSession(ctx, passkeyAuthKeyPrefix+userID)
	if err == nil {
		return session, nil
	}

	challenge := parsedResponse.Response.CollectedClientData.Challenge
	discoverableKey := passkeyAuthKeyPrefix + passkeyAuthDiscoverable + ":" + challenge
	session, err = s.loadSession(ctx, discoverableKey)
	if err != nil {
		return nil, fiberutil.BadRequest("Passkey session expired, please try again")
	}

	return session, nil
}

func (s *PasskeyService) verifyLogin(user *models.WebAuthnUser, session *webauthn.SessionData, parsedResponse *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error) {
	if len(session.UserID) == 0 {
		credential, err := s.webauthn.ValidateDiscoverableLogin(
			func(rawID, userHandle []byte) (webauthn.User, error) {
				return user, nil
			},
			*session,
			parsedResponse,
		)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to verify discoverable passkey login")
			return nil, fiberutil.Unauthorized("Passkey verification failed")
		}

		return credential, nil
	}

	credential, err := s.webauthn.ValidateLogin(user, *session, parsedResponse)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to verify passkey login")
		return nil, fiberutil.Unauthorized("Passkey verification failed")
	}

	return credential, nil
}

func (s *PasskeyService) updatePasskeyUsage(ctx context.Context, rawID []byte, credential *webauthn.Credential) {
	credentialID := base64.RawURLEncoding.EncodeToString(rawID)

	passkey, err := s.repos.Passkey().FindByCredentialID(ctx, credentialID)
	if err != nil {
		s.logger.Error().Err(err).Str("credential_id", credentialID).Msg("Failed to find passkey for usage update")
		return
	}

	passkey.UpdateUsage(credential.Authenticator.SignCount)
	if err := s.repos.Passkey().Update(ctx, passkey); err != nil {
		s.logger.Error().Err(err).Str("passkey_id", passkey.ID).Msg("Failed to update passkey usage")
	}
}

func (s *PasskeyService) getUserAndPasskeys(ctx context.Context, userID string) (*models.User, []models.Passkey, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	if user == nil {
		return nil, nil, fiberutil.NotFound()
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	return user, passkeys, nil
}

func (s *PasskeyService) storeSession(ctx context.Context, key string, session *webauthn.SessionData) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return s.cache.Set(ctx, key, string(data), passkeySessionTTL)
}

func (s *PasskeyService) loadSession(ctx context.Context, key string) (*webauthn.SessionData, error) {
	sessionJSON, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	_ = s.cache.Delete(ctx, key)

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &session, nil
}
