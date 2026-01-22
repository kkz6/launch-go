package repositories

import (
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Repository errors - using fiber error utilities
var (
	ErrScriptNotFound    = fiberutil.NotFound("Script not found")
	ErrExecutionNotFound = fiberutil.NotFound("Execution not found")
)
