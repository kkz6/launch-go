package repositories

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Repository errors - using fiber error utilities
var (
	ErrBackupNotFound          = fiberutil.NotFound("Backup not found")
	ErrStorageProviderNotFound = fiberutil.NotFound("Storage provider not found")
	ErrBackupJobNotFound       = fiberutil.NotFound("Backup job not found")
)
