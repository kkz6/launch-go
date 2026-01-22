package services

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Service-level errors with HTTP status codes
// These errors automatically map to the correct HTTP responses when using fiberctx.Abort()
var (
	// 409 Conflict - resource state conflicts
	ErrPendingDeployment = fiberutil.Conflict("A deployment is already in progress")

	// 400 Bad Request - invalid operations
	ErrRollbackNotSupported      = fiberutil.BadRequest("Rollback is only available for sites with zero downtime deployment enabled")
	ErrInvalidRollbackTarget     = fiberutil.BadRequest("Can only rollback to a finished deployment")
	ErrDeploymentNotBelongToSite = fiberutil.BadRequest("Target deployment does not belong to this site")
	ErrSourceControlNotConnected = fiberutil.BadRequest("Source control is not connected")
	ErrSiteNotInstalled          = fiberutil.BadRequest("Site is not installed")
	ErrBranchMismatch            = fiberutil.BadRequest("Branch mismatch")

	// 401 Unauthorized - authentication issues
	ErrInvalidDeployToken = fiberutil.Unauthorized("Invalid deploy token")
)
