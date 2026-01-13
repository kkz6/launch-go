package repositories

import apperrors "github.com/kkz6/launch-go/internal/pkg/errors"

// Repository errors - re-exported from centralized error package
var (
	ErrSiteNotFound        = apperrors.ErrSiteNotFound
	ErrDeploymentNotFound  = apperrors.ErrDeploymentNotFound
	ErrQueueNotFound       = apperrors.ErrQueueNotFound
	ErrCertificateNotFound = apperrors.ErrCertificateNotFound
	ErrRedirectNotFound    = apperrors.ErrRedirectNotFound
	ErrCommandNotFound     = apperrors.ErrCommandNotFound
	ErrReleaseNotFound     = apperrors.ErrReleaseNotFound
)
