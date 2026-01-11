package services

import "errors"

var (
	ErrStorageProviderHasBackups = errors.New("storage provider has associated backups and cannot be deleted")
	ErrInvalidStorageDriver      = errors.New("invalid storage driver")
	ErrConnectionFailed          = errors.New("failed to connect to storage provider")
	ErrInvalidDispatchToken      = errors.New("invalid dispatch token")
)
