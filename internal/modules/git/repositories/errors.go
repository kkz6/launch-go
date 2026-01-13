package repositories

import "github.com/kkz6/launch-go/internal/pkg/response"

// Repository errors with HTTP status codes
var (
	ErrSourceControlNotFound = response.ErrNotFound("Source control not found")
	ErrRepositoryNotFound    = response.ErrNotFound("Repository not found")
)
