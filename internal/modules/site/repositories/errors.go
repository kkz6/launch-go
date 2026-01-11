package repositories

import "errors"

var (
	ErrSiteNotFound        = errors.New("site not found")
	ErrDeploymentNotFound  = errors.New("deployment not found")
	ErrQueueNotFound       = errors.New("queue not found")
	ErrCertificateNotFound = errors.New("certificate not found")
	ErrRedirectNotFound    = errors.New("redirect not found")
	ErrCommandNotFound     = errors.New("command not found")
	ErrReleaseNotFound     = errors.New("release not found")
)
