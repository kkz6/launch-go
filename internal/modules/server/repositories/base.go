package repositories

import (
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Repository errors - re-exported from centralized error package
var (
	ErrServerNotFound         = apperrors.ErrServerNotFound
	ErrServiceNotFound        = apperrors.ErrServiceNotFound
	ErrFirewallRuleNotFound   = apperrors.ErrFirewallRuleNotFound
	ErrCronNotFound           = apperrors.ErrCronNotFound
	ErrDaemonNotFound         = apperrors.ErrDaemonNotFound
	ErrSSHKeyNotFound         = apperrors.ErrSSHKeyNotFound
	ErrTaskNotFound           = apperrors.ErrTaskNotFound
	ErrMetricNotFound         = apperrors.ErrMetricNotFound
	ErrServerProviderNotFound = apperrors.ErrServerProviderNotFound
	ErrDatabaseNotFound       = apperrors.ErrDatabaseNotFound
	ErrDatabaseUserNotFound   = apperrors.ErrDatabaseUserNotFound
)
