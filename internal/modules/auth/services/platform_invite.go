package services

import (
	"context"
	"time"
)

// PlatformInviteReader is the cross-module dependency the auth module needs to
// consume a platform invite during registration. It is satisfied by the staff
// module's Service (which owns invitations and imports billing/models) and
// injected from main.go via SetPlatformInviteReader, so the auth module never
// imports the staff or billing modules directly. This keeps the dependency
// direction one-way (staff -> auth), avoiding an import cycle.
type PlatformInviteReader interface {
	// PlatformInviteByToken returns (email, trialEndsAt, ok, error). ok=false if
	// the token is unknown, expired, or already accepted. A nil error with
	// ok=false is the normal "no usable invite" outcome.
	PlatformInviteByToken(ctx context.Context, token string) (email string, trialEndsAt time.Time, ok bool, err error)

	// AcceptPlatformInviteWithTrial marks the invite accepted and creates an
	// on_trial subscription for the given personal team, atomically.
	AcceptPlatformInviteWithTrial(ctx context.Context, token, personalTeamID string) error
}

// SetPlatformInviteReader wires the cross-module reader used to consume a
// platform invite at registration time. Injected from main.go, mirroring how
// the staff module wires its own cross-module dependencies. When nil, the
// register flow simply skips the trial-grant step.
func (s *AuthService) SetPlatformInviteReader(r PlatformInviteReader) {
	s.platformInvites = r
}
