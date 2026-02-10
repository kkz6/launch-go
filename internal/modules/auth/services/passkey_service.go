package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

	wauthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: rpName,
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Timeout:    time.Duration(cfg.Passkey.Timeout) * time.Millisecond,
				TimeoutUVD: time.Duration(cfg.Passkey.Timeout) * time.Millisecond,
			},
			Registration: webauthn.TimeoutConfig{
				Timeout:    time.Duration(cfg.Passkey.Timeout) * time.Millisecond,
				TimeoutUVD: time.Duration(cfg.Passkey.Timeout) * time.Millisecond,
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
		return nil, errors.New("maximum number of passkeys reached")
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	// Exclude existing credentials from registration
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
		return nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	if err := s.storeSession(ctx, passkeyRegKeyPrefix+userID, session); err != nil {
		return nil, err
	}

	return creation, nil
}

// FinishRegistration completes WebAuthn registration
func (s *PasskeyService) FinishRegistration(ctx context.Context, userID string, body []byte, name *string) (*models.Passkey, error) {
	user, passkeys, err := s.getUserAndPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	session, err := s.loadSession(ctx, passkeyRegKeyPrefix+userID, "registration session expired or not found")
	if err != nil {
		return nil, err
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	credential, err := s.webauthn.CreateCredential(webAuthnUser, *session, parsedResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to verify registration: %w", err)
	}

	passkey := models.PasskeyFromCredential(userID, credential, name)
	if err := s.repos.Passkey().Create(ctx, passkey); err != nil {
		return nil, fmt.Errorf("failed to save passkey: %w", err)
	}

	return passkey, nil
}

// BeginLogin generates WebAuthn authentication options
func (s *PasskeyService) BeginLogin(ctx context.Context, email string) (*protocol.CredentialAssertion, error) {
	var (
		assertion *protocol.CredentialAssertion
		session   *webauthn.SessionData
		err       error
		cacheKey  string
	)

	if email != "" {
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
		assertion, session, err = s.webauthn.BeginLogin(webAuthnUser)
		if err != nil {
			return nil, fiberutil.BadRequest("Failed to initialize passkey authentication")
		}

		cacheKey = passkeyAuthKeyPrefix + user.ID
	} else {
		assertion, session, err = s.webauthn.BeginDiscoverableLogin()
		if err != nil {
			return nil, fiberutil.BadRequest("Failed to initialize passkey authentication")
		}

		cacheKey = passkeyAuthKeyPrefix + passkeyAuthDiscoverable + ":" + session.Challenge
	}

	if err := s.storeSession(ctx, cacheKey, session); err != nil {
		return nil, fiberutil.BadRequest("Failed to start passkey authentication session")
	}

	return assertion, nil
}

// FinishLogin completes WebAuthn authentication and returns the authenticated user
func (s *PasskeyService) FinishLogin(ctx context.Context, body []byte) (*models.User, error) {
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fiberutil.BadRequest("Invalid passkey response")
	}

	// Always look up the passkey by credential ID to get the real user ID.
	// We cannot rely on userHandle because EncodeUserIDAsString may cause
	// the browser to return garbled bytes that don't match the original user ID.
	credentialID := base64.RawURLEncoding.EncodeToString(parsedResponse.RawID)
	s.logger.Debug().Str("credential_id", credentialID).Msg("Passkey login: looking up credential")

	passkey, err := s.repos.Passkey().FindByCredentialID(ctx, credentialID)
	if err != nil {
		s.logger.Warn().Err(err).Str("credential_id", credentialID).Msg("Passkey login: credential not found")
		return nil, fiberutil.Unauthorized("Passkey not recognized")
	}

	userID := passkey.UserID
	s.logger.Debug().Str("user_id", userID).Str("credential_id", credentialID).Msg("Passkey login: credential found")

	// Try user-specific session first, then discoverable session
	cacheKey := passkeyAuthKeyPrefix + userID
	session, err := s.loadSession(ctx, cacheKey, "")
	if err != nil {
		challenge := parsedResponse.Response.CollectedClientData.Challenge
		discoverableKey := passkeyAuthKeyPrefix + passkeyAuthDiscoverable + ":" + challenge
		session, err = s.loadSession(ctx, discoverableKey, "")
		if err != nil {
			s.logger.Warn().Str("user_id", userID).Msg("Passkey login: session not found")
			return nil, fiberutil.BadRequest("Passkey session expired, please try again")
		}
	}

	s.logger.Debug().Str("user_id", userID).Msg("Passkey login: session found, verifying")
	return s.finishLoginWithSession(ctx, userID, session, parsedResponse)
}

// updatePasskeyAfterLogin updates the passkey sign count and last used timestamp
func (s *PasskeyService) updatePasskeyAfterLogin(ctx context.Context, rawID []byte, credential *webauthn.Credential) {
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
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(sessionData), passkeySessionTTL); err != nil {
		return fmt.Errorf("failed to store session data: %w", err)
	}

	return nil
}

func (s *PasskeyService) parseSession(sessionJSON string) (*webauthn.SessionData, error) {
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &session, nil
}

func (s *PasskeyService) loadSession(ctx context.Context, key, notFoundMsg string) (*webauthn.SessionData, error) {
	sessionJSON, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, errors.New(notFoundMsg)
	}

	if err := s.cache.Delete(ctx, key); err != nil && s.logger != nil {
		s.logger.Warn().Err(err).Msg("Failed to delete passkey session")
	}

	return s.parseSession(sessionJSON)
}

func (s *PasskeyService) finishLoginWithSession(ctx context.Context, userID string, session *webauthn.SessionData, parsedResponse *protocol.ParsedCredentialAssertionData) (*models.User, error) {
	user, passkeys, err := s.getUserAndPasskeysForLogin(ctx, userID)
	if err != nil {
		return nil, err
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	credential, err := s.verifyLogin(webAuthnUser, session, parsedResponse)
	if err != nil {
		return nil, err
	}

	s.updatePasskeyAfterLogin(ctx, parsedResponse.RawID, credential)

	return user, nil
}

func (s *PasskeyService) getUserAndPasskeysForLogin(ctx context.Context, userID string) (*models.User, []models.Passkey, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, nil, fiberutil.Unauthorized("User not found")
	}

	if user == nil {
		return nil, nil, fiberutil.Unauthorized("User not found")
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, userID)
	if err != nil {
		return nil, nil, fiberutil.BadRequest("Failed to retrieve passkeys")
	}

	return user, passkeys, nil
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

	s.logger.Debug().
		Str("session_user_id", string(session.UserID)).
		Str("webauthn_user_id", string(user.WebAuthnID())).
		Str("response_user_handle", base64.RawURLEncoding.EncodeToString(parsedResponse.Response.UserHandle)).
		Int("credential_count", len(user.WebAuthnCredentials())).
		Msg("Passkey login: validating")

	credential, err := s.webauthn.ValidateLogin(user, *session, parsedResponse)
	if err != nil {
		s.logger.Error().Err(err).
			Str("session_user_id", string(session.UserID)).
			Str("webauthn_user_id", string(user.WebAuthnID())).
			Str("response_user_handle", base64.RawURLEncoding.EncodeToString(parsedResponse.Response.UserHandle)).
			Msg("Failed to verify passkey login")
		return nil, fiberutil.Unauthorized("Passkey verification failed")
	}

	return credential, nil
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

	if passkey == nil {
		return fiberutil.NotFound()
	}

	if passkey.UserID != userID {
		return fiberutil.NotFound()
	}

	passkey.Name = &name

	return s.repos.Passkey().Update(ctx, passkey)
}

// DeletePasskey deletes a passkey for a user
func (s *PasskeyService) DeletePasskey(ctx context.Context, passkeyID, userID string) error {
	return s.repos.Passkey().DeleteByUserID(ctx, passkeyID, userID)
}
