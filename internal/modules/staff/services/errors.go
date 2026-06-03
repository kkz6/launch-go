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

// ErrUserNotFound is returned by SetUserStatus when the target user does not
// exist. The handler maps it to 404.
var ErrUserNotFound = errors.New("user not found")

// ErrCannotSuspendStaff is returned when a suspend/unsuspend action targets a
// user who holds a staff role. Staff members (and peers) must not be lockable
// through this endpoint. The handler maps it to 409.
var ErrCannotSuspendStaff = errors.New("cannot suspend a staff member")

// ErrCannotSuspendSelf is returned when a staff member targets their own
// account, which would lock themselves out. The handler maps it to 409.
var ErrCannotSuspendSelf = errors.New("cannot change your own account status")
