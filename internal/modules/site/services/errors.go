package services

import (
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Service-level errors with HTTP status codes
// These errors automatically map to the correct HTTP responses when using response.Abort()
var (
	// 409 Conflict - resource state conflicts
	ErrPendingDeployment = apperrors.Conflict("A deployment is already in progress")

	// 400 Bad Request - invalid operations
	ErrRollbackNotSupported      = apperrors.BadRequest("Rollback is only available for sites with zero downtime deployment enabled")
	ErrInvalidRollbackTarget     = apperrors.BadRequest("Can only rollback to a finished deployment")
	ErrDeploymentNotBelongToSite = apperrors.BadRequest("Target deployment does not belong to this site")
	ErrSourceControlNotConnected = apperrors.BadRequest("Source control is not connected")
	ErrSiteNotInstalled          = apperrors.BadRequest("Site is not installed")
	ErrBranchMismatch            = apperrors.BadRequest("Branch mismatch")

	// 401 Unauthorized - authentication issues
	ErrInvalidDeployToken = apperrors.Unauthorized("Invalid deploy token")
)
