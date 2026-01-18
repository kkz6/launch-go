package repositories

import apperrors "github.com/kkz6/launch-go/internal/pkg/errors"

// Repository errors - re-exported from centralized error package
var (
	ErrScriptNotFound    = apperrors.ErrScriptNotFound
	ErrExecutionNotFound = apperrors.ErrExecutionNotFound
)
