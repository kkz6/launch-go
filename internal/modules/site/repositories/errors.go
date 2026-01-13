package repositories

import "github.com/kkz6/launch-go/internal/pkg/response"

// Repository errors with HTTP status codes
var (
	ErrSiteNotFound        = response.ErrNotFound("Site not found")
	ErrDeploymentNotFound  = response.ErrNotFound("Deployment not found")
	ErrQueueNotFound       = response.ErrNotFound("Queue not found")
	ErrCertificateNotFound = response.ErrNotFound("Certificate not found")
	ErrRedirectNotFound    = response.ErrNotFound("Redirect not found")
	ErrCommandNotFound     = response.ErrNotFound("Command not found")
	ErrReleaseNotFound     = response.ErrNotFound("Release not found")
)
