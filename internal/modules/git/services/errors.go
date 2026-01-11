package services

import "errors"

var (
	ErrProviderNotSupported = errors.New("provider not supported")
	ErrNoInstallationID     = errors.New("no installation ID found")
	ErrHasSites             = errors.New("cannot delete source control with associated sites")
)
