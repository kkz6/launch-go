package middleware

// SessionValidator reports whether a session id still refers to a live,
// non-revoked session. Returning false rejects the bearer token at the auth
// chokepoint, making logout / "log out other devices" / session revocation
// take effect immediately on the access token instead of lingering until it
// expires.
type SessionValidator func(sessionID string) bool

var sessionValidatorMiddleware struct {
	validate SessionValidator
}

// InitSessionValidator wires the session-liveness lookup. Call during bootstrap.
// When wired, every session-bound access token is checked against it on each
// authenticated request, mirroring the revocation check the refresh path
// already performs.
func InitSessionValidator(validate SessionValidator) {
	sessionValidatorMiddleware.validate = validate
}

// sessionIsLive reports whether the access token's backing session is still
// valid. Fail-open when no validator is wired (tests / contexts without the
// auth repo) so behavior matches the pre-existing access path; the validator is
// always wired in production.
func sessionIsLive(sessionID string) bool {
	if sessionValidatorMiddleware.validate == nil {
		return true
	}
	return sessionValidatorMiddleware.validate(sessionID)
}
