package migrations

import (
	"time"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{ID: "0071_11_17_000000_github_repository_grants", Name: "Add verified GitHub connections and explicit repository grants", Timestamp: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC), Up: githubRepositoryGrantsUp})
}

func githubRepositoryGrantsUp(db *gorm.DB) error {
	// UserID is historical attribution; sponsor authority is a separate row.
	// Deleting a former creator must not delete another owner's team connection.
	if db.Migrator().HasConstraint(&sourceControlWithFK{}, "User") {
		if err := db.Migrator().DropConstraint(&sourceControlWithFK{}, "User"); err != nil {
			return err
		}
	}
	// Additive migration preserves local repository IDs and workload references.
	// Legacy inventory never becomes a grant: every old GitHub connection needs
	// a verified identity and explicit selection before it can issue credentials.
	for _, entry := range []struct {
		model  any
		fields []string
	}{
		{&gitmodels.SourceControl{}, []string{"Status", "Generation", "SelectionVersion", "GitHubAuthorizationID", "GitHubInstallationID"}},
		{&gitmodels.SourceControlRepository{}, []string{"GitHubRepositoryID", "Selected", "Available", "GrantVersion", "VerifiedAt"}},
	} {
		for _, field := range entry.fields {
			if !db.Migrator().HasColumn(entry.model, field) {
				if err := db.Migrator().AddColumn(entry.model, field); err != nil {
					return err
				}
			}
		}
	}
	for _, model := range []any{&gitmodels.GitHubAuthorization{}, &gitmodels.GitHubConnectionAttempt{}, &gitmodels.GitHubInstallation{}, &gitmodels.GitHubInstallationRepository{}, &gitmodels.GitHubCredentialLease{}, &gitmodels.GitHubCleanupManifest{}} {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
	}
	return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS github_connection_repository_id ON source_control_repositories (source_control_id, git_hub_repository_id) WHERE git_hub_repository_id <> ''").Error
}
