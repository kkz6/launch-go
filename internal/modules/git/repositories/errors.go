package repositories

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Repository errors - using fiber error utilities
var (
	ErrSourceControlNotFound = fiberutil.NotFound("Source control not found")
	ErrRepositoryNotFound    = fiberutil.NotFound("Repository not found")
)
