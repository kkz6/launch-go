package services

import "errors"

var (
	ErrPendingDeployment         = errors.New("a deployment is already in progress")
	ErrRollbackNotSupported      = errors.New("rollback is only available for sites with zero downtime deployment enabled")
	ErrInvalidRollbackTarget     = errors.New("can only rollback to a finished deployment")
	ErrDeploymentNotBelongToSite = errors.New("target deployment does not belong to this site")
	ErrSourceControlNotConnected = errors.New("source control is not connected")
	ErrSiteNotInstalled          = errors.New("site is not installed")
)
