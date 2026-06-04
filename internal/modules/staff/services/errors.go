package services

import (
	"errors"
	"fmt"
)

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

// ErrServerNotFound is returned by GetServerDetail when no server matches the
// id. The handler maps it to 404.
var ErrServerNotFound = errors.New("server not found")

// ErrFailureNotFound is returned by FailureLog when no failure matches the
// kind+id. The handler maps it to 404.
var ErrFailureNotFound = errors.New("failure not found")

// ErrCannotSuspendStaff is returned when a suspend/unsuspend action targets a
// user who holds a staff role. Staff members (and peers) must not be lockable
// through this endpoint. The handler maps it to 409.
var ErrCannotSuspendStaff = errors.New("cannot suspend a staff member")

// ErrCannotSuspendSelf is returned when a staff member targets their own
// account, which would lock themselves out. The handler maps it to 409.
var ErrCannotSuspendSelf = errors.New("cannot change your own account status")

// ErrCannotDeleteSelf is returned when a staff member targets their own account
// for deletion. The handler maps it to 409.
var ErrCannotDeleteSelf = errors.New("cannot delete your own account")

// ErrUserAlreadyExists is returned by InviteUser when a registered user already
// holds the invited email. The handler maps it to 409.
var ErrUserAlreadyExists = errors.New("a user with this email already exists")

// ErrInvitePending is returned by InviteUser when a pending (non-accepted,
// non-expired) invitation already exists for the email. The handler maps it to
// 409.
var ErrInvitePending = errors.New("a pending invitation already exists for this email")

// ErrInvalidTrialDate is returned by InviteUser when the trial end date is not
// in the future. The handler maps it to 400.
var ErrInvalidTrialDate = errors.New("trial end date must be in the future")

// ErrInvalidInviteEmail is returned by InviteUser when the email is empty. The
// handler maps it to 400.
var ErrInvalidInviteEmail = errors.New("email is required")

// ErrEmailSenderNotConfigured is returned when the platform-invitation email
// transport has not been wired into the staff service.
var ErrEmailSenderNotConfigured = errors.New("email sender not configured")

// NotDeletableError carries the human-readable reason a user cannot be deleted
// (holds a staff role, has a paid order, has/had a paid subscription). The
// handler surfaces Reason in a 409 response. It is the typed error returned by
// DeleteUser when the in-transaction re-check refuses the delete, and is also
// constructed from the reason CanDeleteUser reports.
type NotDeletableError struct {
	Reason string
}

func (e *NotDeletableError) Error() string {
	return fmt.Sprintf("user not deletable: %s", e.Reason)
}

// newNotDeletableError builds a NotDeletableError for the given reason.
func newNotDeletableError(reason string) *NotDeletableError {
	return &NotDeletableError{Reason: reason}
}
