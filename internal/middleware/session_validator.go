package middleware

import "github.com/kkz6/launch-go/internal/pkg/security"

// SessionValidator reports whether a session id still refers to a live,
// non-revoked session. Returning false rejects the bearer token at the auth
// chokepoint, making logout / "log out other devices" / session revocation
// take effect immediately on the access token instead of lingering until it
// expires.
type SessionValidator func(sessionID string) bool

// InitSessionValidator wires the session-liveness lookup. Call during bootstrap.
// When wired, every session-bound access token is checked against it on each
// authenticated request, mirroring the revocation check the refresh path
// already performs.
func InitSessionValidator(validate SessionValidator) {
	security.SetSessionValidator(validate)
}
