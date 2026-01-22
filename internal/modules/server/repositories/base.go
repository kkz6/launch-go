package repositories

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Repository errors - using fiberutil error package
var (
	ErrServerNotFound         = fiberutil.NotFound("Server not found")
	ErrServiceNotFound        = fiberutil.NotFound("Service not found")
	ErrFirewallRuleNotFound   = fiberutil.NotFound("Firewall rule not found")
	ErrCronNotFound           = fiberutil.NotFound("Cron job not found")
	ErrDaemonNotFound         = fiberutil.NotFound("Daemon not found")
	ErrSSHKeyNotFound         = fiberutil.NotFound("SSH key not found")
	ErrTaskNotFound           = fiberutil.NotFound("Task not found")
	ErrMetricNotFound         = fiberutil.NotFound("Metric not found")
	ErrServerProviderNotFound = fiberutil.NotFound("Server provider not found")
	ErrDatabaseNotFound       = fiberutil.NotFound("Database not found")
	ErrDatabaseUserNotFound   = fiberutil.NotFound("Database user not found")
)
