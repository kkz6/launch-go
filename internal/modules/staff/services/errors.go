package services

import "errors"

// ErrServerLogReaderUnavailable is returned when the cross-module server log
// reader has not been wired into the staff service.
var ErrServerLogReaderUnavailable = errors.New("server log reader not configured")
