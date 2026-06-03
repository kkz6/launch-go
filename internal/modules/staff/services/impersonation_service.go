package services

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/kkz6/launch-go/internal/modules/staff/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// impersonationTTL bounds how long a minted spectate token is valid. Keep this
// short: the token authenticates AS the customer, so a leaked token is a
// full account-takeover until it expires.
const impersonationTTL = 30 * time.Minute

// StartImpersonation begins a read-only "spectate as user" session: it records
// an audit row and mints a short-lived JWT that authenticates AS the target
// user while carrying the impersonator's identity for audit + exit.
func (s *Service) StartImpersonation(ctx context.Context, staffID, targetUserID, reason string) (token string, session *models.ImpersonationSession, err error) {
	if staffID == "" || targetUserID == "" {
		return "", nil, ErrInvalidImpersonationRequest
	}

	if s.jwtSecret == "" {
		return "", nil, ErrJWTSecretNotConfigured
	}

	target, err := s.repos.UserByID(ctx, targetUserID)
	if err != nil {
		return "", nil, err
	}

	if target == nil {
		return "", nil, ErrTargetUserNotFound
	}

	session = &models.ImpersonationSession{
		StaffID:      staffID,
		TargetUserID: target.ID,
		TeamID:       target.CurrentTeamID,
		Reason:       nilIfEmpty(reason),
	}

	if err := s.repos.CreateImpersonationSession(ctx, session); err != nil {
		return "", nil, err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"jti":   util.NewULID(),
		"sub":   target.ID,
		"email": target.Email,
		"name":  target.Name,
		"type":  "access",
		"iat":   now.Unix(),
		"exp":   now.Add(impersonationTTL).Unix(),

		"impersonator_id":   staffID,
		"impersonation_sid": session.ID,
		"read_only":         true,
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := jwtToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", nil, err
	}

	return signed, session, nil
}

// StopImpersonation ends an active spectate session by stamping ended_at.
func (s *Service) StopImpersonation(ctx context.Context, sessionID string) error {
	return s.repos.EndImpersonationSession(ctx, sessionID, time.Now())
}

// StopImpersonationForStaff ends the authenticated staff member's active
// spectate session. The exit flow is driven with the STAFF token (not the
// impersonation token), so the session is derived from the staff identity
// rather than trusting a session id from the request. Returns (false, nil)
// when the staff member has no active session — a clean no-op for the caller
// to surface as a success.
func (s *Service) StopImpersonationForStaff(ctx context.Context, staffID string) (stopped bool, err error) {
	if staffID == "" {
		return false, ErrInvalidImpersonationRequest
	}

	session, err := s.repos.ActiveImpersonationForStaff(ctx, staffID)
	if err != nil {
		return false, err
	}

	if session == nil {
		return false, nil
	}

	if err := s.repos.EndImpersonationSession(ctx, session.ID, time.Now()); err != nil {
		return false, err
	}

	return true, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
