package repositories

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Repository errors - using fiber error utilities
var (
	ErrSiteNotFound        = fiberutil.NotFound("Site not found")
	ErrDeploymentNotFound  = fiberutil.NotFound("Deployment not found")
	ErrQueueNotFound       = fiberutil.NotFound("Queue not found")
	ErrCertificateNotFound = fiberutil.NotFound("Certificate not found")
	ErrRedirectNotFound    = fiberutil.NotFound("Redirect not found")
	ErrCommandNotFound     = fiberutil.NotFound("Command not found")
)
