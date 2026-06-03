package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
)

// invitationValidity is how long a platform invitation remains usable before it
// expires.
const invitationValidity = 14 * 24 * time.Hour

// inviteTokenBytes is the number of random bytes used to build an invite token.
// 32 bytes of crypto/rand encoded base64url yields a 43-char URL-safe string,
// comfortably above the 32-char minimum and well within the column's 64 chars.
const inviteTokenBytes = 32

// InviteUser creates a platform invitation for the given email with a trial
// running until trialEndsAt, then emails the recipient an invite link.
//
// Guards, in order:
//   - the email must be non-empty (ErrInvalidInviteEmail);
//   - trialEndsAt must be in the future (ErrInvalidTrialDate);
//   - no registered user may already hold the email (ErrUserAlreadyExists);
//   - no pending invitation may already exist for the email (ErrInvitePending).
//
// Email-send failures do NOT roll back the invite: the row is still valid and
// usable (an admin can re-send or share the link), so the send error is logged
// via the returned error path only when no other work succeeded. Here we keep
// the row and surface the send error to the caller so the failure is visible,
// while still returning the created invitation.
func (s *Service) InviteUser(ctx context.Context, email string, trialEndsAt time.Time, invitedBy string) (*authmodels.PlatformInvitation, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, ErrInvalidInviteEmail
	}

	now := time.Now()
	if !trialEndsAt.After(now) {
		return nil, ErrInvalidTrialDate
	}

	exists, err := s.repos.UserExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	pending, err := s.repos.FindPendingInvitationByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return nil, ErrInvitePending
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	invitation := &authmodels.PlatformInvitation{
		Email:       email,
		Token:       token,
		TrialEndsAt: trialEndsAt,
		InvitedBy:   invitedBy,
		ExpiresAt:   now.Add(invitationValidity),
	}

	if err := s.repos.CreatePlatformInvitation(ctx, invitation); err != nil {
		return nil, err
	}

	if err := s.sendInvitationEmail(ctx, email, token, trialEndsAt); err != nil {
		// The invite row is valid and usable; surface the transport error so the
		// admin knows the email did not go out, but keep the created invitation.
		return invitation, fmt.Errorf("invitation created but email delivery failed: %w", err)
	}

	return invitation, nil
}

// sendInvitationEmail builds the invite link and delivers the invitation email.
func (s *Service) sendInvitationEmail(ctx context.Context, email, token string, trialEndsAt time.Time) error {
	if s.emailSender == nil {
		return ErrEmailSenderNotConfigured
	}

	inviteURL := fmt.Sprintf("%s/register?invite=%s", strings.TrimRight(s.frontendURL, "/"), token)

	html, _, err := templates.PlatformInvitationEmail(inviteURL, trialEndsAt)
	if err != nil {
		return err
	}

	return s.emailSender.Send(ctx, email, "You're invited to try Launch", html, true)
}

// ListPendingInvitations returns a page of non-accepted, non-expired
// invitations together with the total count.
func (s *Service) ListPendingInvitations(ctx context.Context, limit, offset int) ([]authmodels.PlatformInvitation, int64, error) {
	return s.repos.ListPendingInvitations(ctx, limit, offset)
}

// RevokeInvitation deletes a pending invitation by id. Returns
// gorm.ErrRecordNotFound (via the repo) when no row matched.
func (s *Service) RevokeInvitation(ctx context.Context, id string) error {
	return s.repos.DeleteInvitation(ctx, id)
}

// generateInviteToken returns a random URL-safe token built from 32 bytes of
// crypto/rand, base64url-encoded (no padding). The column carries a unique
// index; the entropy makes a collision astronomically unlikely.
func generateInviteToken() (string, error) {
	buf := make([]byte, inviteTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
