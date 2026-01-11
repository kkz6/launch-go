package repositories

import "errors"

var (
	ErrSourceControlNotFound = errors.New("source control not found")
	ErrRepositoryNotFound    = errors.New("repository not found")
)
