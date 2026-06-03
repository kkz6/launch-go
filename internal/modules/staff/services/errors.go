package services

import "errors"

// ErrServerLogReaderUnavailable is returned when the cross-module server log
// reader has not been wired into the staff service.
var ErrServerLogReaderUnavailable = errors.New("server log reader not configured")

// ErrInvalidImpersonationRequest is returned when an impersonation request is
// missing a required identifier: StartImpersonation needs both a staff id and a
// target user id, while StopImpersonationForStaff needs a staff id.
var ErrInvalidImpersonationRequest = errors.New("missing required impersonation identifiers")

// ErrTargetUserNotFound is returned when the impersonation target does not exist.
var ErrTargetUserNotFound = errors.New("target user not found")

// ErrJWTSecretNotConfigured is returned when the impersonation flow is invoked
// without a configured signing secret. Failing closed prevents minting tokens
// signed with an empty key.
var ErrJWTSecretNotConfigured = errors.New("jwt secret not configured")
