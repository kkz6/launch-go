package repositories

import "errors"

var (
	ErrBackupNotFound          = errors.New("backup not found")
	ErrStorageProviderNotFound = errors.New("storage provider not found")
	ErrBackupJobNotFound       = errors.New("backup job not found")
)
