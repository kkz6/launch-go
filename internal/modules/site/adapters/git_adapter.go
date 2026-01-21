package adapters

import (
	"context"

	gitcontracts "github.com/kkz6/launch-go/internal/modules/git/contracts"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
)

// GitReaderAdapter adapts git repositories to the GitReader interface
type GitReaderAdapter struct {
	sourceControlRepo gitcontracts.SourceControlRepository
	repoRepo          gitcontracts.SourceControlRepoRepository
}

// NewGitReaderAdapter creates a new GitReaderAdapter
func NewGitReaderAdapter(sourceControlRepo gitcontracts.SourceControlRepository, repoRepo gitcontracts.SourceControlRepoRepository) contracts.GitReader {
	return &GitReaderAdapter{
		sourceControlRepo: sourceControlRepo,
		repoRepo:          repoRepo,
	}
}

// FindSourceControlByID retrieves a source control by ID
func (a *GitReaderAdapter) FindSourceControlByID(ctx context.Context, id string) (*gitmodels.SourceControl, error) {
	return a.sourceControlRepo.FindByID(ctx, id)
}

// FindRepositoryByID retrieves a source control repository by ID
func (a *GitReaderAdapter) FindRepositoryByID(ctx context.Context, id string) (*gitmodels.SourceControlRepository, error) {
	return a.repoRepo.FindRepositoryByID(ctx, id)
}
