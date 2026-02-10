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
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
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
	repos    *repositories.Registry
	config   *config.Config
	cache    cache.Cache
	webauthn *webauthn.WebAuthn
	logger   *zerolog.Logger
}

// NewPasskeyService creates a new PasskeyService instance
func NewPasskeyService(repos *repositories.Registry, cfg *config.Config, c cache.Cache, logger *zerolog.Logger) (*PasskeyService, error) {
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
		EncodeUserIDAsString: true,
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
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, userID)
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

	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session data: %w", err)
	}

	if err := s.cache.Set(ctx, passkeyRegKeyPrefix+userID, string(sessionData), passkeySessionTTL); err != nil {
		return nil, fmt.Errorf("failed to store session data: %w", err)
	}

	return creation, nil
}

// FinishRegistration completes WebAuthn registration
func (s *PasskeyService) FinishRegistration(ctx context.Context, userID string, body []byte, name *string) (*models.Passkey, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Retrieve and delete session data
	sessionJSON, err := s.cache.Get(ctx, passkeyRegKeyPrefix+userID)
	if err != nil {
		return nil, errors.New("registration session expired or not found")
	}

	_ = s.cache.Delete(ctx, passkeyRegKeyPrefix+userID)

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w", err)
	}

	credential, err := s.webauthn.CreateCredential(webAuthnUser, session, parsedResponse)
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
			return nil, err
		}

		if len(passkeys) == 0 {
			return nil, errors.New("no passkeys registered for this account")
		}

		webAuthnUser := models.NewWebAuthnUser(user, passkeys)
		assertion, session, err = s.webauthn.BeginLogin(webAuthnUser)
		if err != nil {
			return nil, fmt.Errorf("failed to begin login: %w", err)
		}

		cacheKey = passkeyAuthKeyPrefix + user.ID
	} else {
		assertion, session, err = s.webauthn.BeginDiscoverableLogin()
		if err != nil {
			return nil, fmt.Errorf("failed to begin discoverable login: %w", err)
		}

		cacheKey = passkeyAuthKeyPrefix + passkeyAuthDiscoverable + ":" + session.Challenge
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session data: %w", err)
	}

	if err := s.cache.Set(ctx, cacheKey, string(sessionData), passkeySessionTTL); err != nil {
		return nil, fmt.Errorf("failed to store session data: %w", err)
	}

	return assertion, nil
}

// FinishLogin completes WebAuthn authentication and returns the authenticated user
func (s *PasskeyService) FinishLogin(ctx context.Context, body []byte) (*models.User, error) {
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse login response: %w", err)
	}

	// Try discoverable login first by checking if userHandle is present
	if len(parsedResponse.Response.UserHandle) > 0 {
		userID := string(parsedResponse.Response.UserHandle)

		// Try user-specific session first
		cacheKey := passkeyAuthKeyPrefix + userID
		sessionJSON, err := s.cache.Get(ctx, cacheKey)

		if err != nil {
			// Try discoverable sessions by looking up credential
			credentialID := base64.RawURLEncoding.EncodeToString(parsedResponse.RawID)
			passkey, findErr := s.repos.Passkey().FindByCredentialID(ctx, credentialID)
			if findErr != nil {
				return nil, fiberutil.Unauthorized()
			}

			cacheKey = passkeyAuthKeyPrefix + passkey.UserID
			sessionJSON, err = s.cache.Get(ctx, cacheKey)
			if err != nil {
				return nil, errors.New("login session expired or not found")
			}
		}

		_ = s.cache.Delete(ctx, cacheKey)

		var session webauthn.SessionData
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
		}

		user, err := s.repos.User().FindByID(ctx, userID)
		if err != nil || user == nil {
			return nil, fiberutil.Unauthorized()
		}

		passkeys, err := s.repos.Passkey().FindByUserID(ctx, user.ID)
		if err != nil {
			return nil, err
		}

		webAuthnUser := models.NewWebAuthnUser(user, passkeys)

		if len(session.UserID) == 0 {
			// Discoverable login
			credential, err := s.webauthn.ValidateDiscoverableLogin(
				func(rawID, userHandle []byte) (webauthn.User, error) {
					return webAuthnUser, nil
				},
				session,
				parsedResponse,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to verify discoverable login: %w", err)
			}

			s.updatePasskeyAfterLogin(ctx, parsedResponse.RawID, credential)

			return user, nil
		}

		// User-specific login
		credential, err := s.webauthn.ValidateLogin(webAuthnUser, session, parsedResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to verify login: %w", err)
		}

		s.updatePasskeyAfterLogin(ctx, parsedResponse.RawID, credential)

		return user, nil
	}

	// Fallback: try to find the user by credential ID
	credentialID := base64.RawURLEncoding.EncodeToString(parsedResponse.RawID)
	passkey, err := s.repos.Passkey().FindByCredentialID(ctx, credentialID)
	if err != nil {
		return nil, fiberutil.Unauthorized()
	}

	cacheKey := passkeyAuthKeyPrefix + passkey.UserID
	sessionJSON, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, errors.New("login session expired or not found")
	}

	_ = s.cache.Delete(ctx, cacheKey)

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	user, err := s.repos.User().FindByID(ctx, passkey.UserID)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	passkeys, err := s.repos.Passkey().FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	webAuthnUser := models.NewWebAuthnUser(user, passkeys)
	credential, err := s.webauthn.ValidateLogin(webAuthnUser, session, parsedResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to verify login: %w", err)
	}

	s.updatePasskeyAfterLogin(ctx, parsedResponse.RawID, credential)

	return user, nil
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
