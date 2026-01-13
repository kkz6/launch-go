package repositories

import "github.com/kkz6/launch-go/internal/pkg/response"

// Repository errors with HTTP status codes
var (
	ErrBackupNotFound          = response.ErrNotFound("Backup not found")
	ErrStorageProviderNotFound = response.ErrNotFound("Storage provider not found")
	ErrBackupJobNotFound       = response.ErrNotFound("Backup job not found")
)
