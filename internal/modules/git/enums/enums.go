package enums

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// GitProviderType represents the type of git provider
type GitProviderType string

const (
	GitProviderGitHub    GitProviderType = "github"
	GitProviderGitLab    GitProviderType = "gitlab"
	GitProviderBitbucket GitProviderType = "bitbucket"
)

// AllGitProviders returns all available git providers
func AllGitProviders() []GitProviderType {
	return []GitProviderType{
		GitProviderGitHub,
		GitProviderGitLab,
		GitProviderBitbucket,
	}
}

// String returns the string representation of the provider
func (p GitProviderType) String() string {
	return string(p)
}

// Label returns a human-readable label for the provider
func (p GitProviderType) Label() string {
	switch p {
	case GitProviderGitHub:
		return "GitHub"
	case GitProviderGitLab:
		return "GitLab"
	case GitProviderBitbucket:
		return "Bitbucket"
	default:
		return strings.Title(string(p))
	}
}

// IsValid checks if the provider type is valid
func (p GitProviderType) IsValid() bool {
	switch p {
	case GitProviderGitHub, GitProviderGitLab, GitProviderBitbucket:
		return true
	default:
		return false
	}
}

// ParseGitProviderType parses a string into a GitProviderType
func ParseGitProviderType(s string) (GitProviderType, error) {
	provider := GitProviderType(strings.ToLower(s))

	if !provider.IsValid() {
		return "", fmt.Errorf("invalid git provider type: %s", s)
	}

	return provider, nil
}

// Value implements driver.Valuer for database storage
func (p GitProviderType) Value() (driver.Value, error) {
	return string(p), nil
}

// Scan implements sql.Scanner for database retrieval
func (p *GitProviderType) Scan(value interface{}) error {
	if value == nil {
		*p = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*p = GitProviderType(v)
	case []byte:
		*p = GitProviderType(string(v))
	default:
		return fmt.Errorf("cannot scan type %T into GitProviderType", value)
	}

	return nil
}

// AccountType represents the type of git account (user or organization)
type AccountType string

const (
	AccountTypeUser         AccountType = "user"
	AccountTypeOrganization AccountType = "organization"
)

// String returns the string representation of the account type
func (a AccountType) String() string {
	return string(a)
}

// IsValid checks if the account type is valid
func (a AccountType) IsValid() bool {
	switch a {
	case AccountTypeUser, AccountTypeOrganization:
		return true
	default:
		return false
	}
}

// IsOrganization checks if the account is an organization
func (a AccountType) IsOrganization() bool {
	return strings.ToLower(string(a)) == string(AccountTypeOrganization)
}

// IsUser checks if the account is a user
func (a AccountType) IsUser() bool {
	return strings.ToLower(string(a)) == string(AccountTypeUser)
}

// ParseAccountType parses a string into an AccountType
func ParseAccountType(s string) AccountType {
	lower := strings.ToLower(s)

	switch lower {
	case "organization", "org":
		return AccountTypeOrganization
	default:
		return AccountTypeUser
	}
}

// RepositorySelection represents how repositories are selected for an installation
type RepositorySelection string

const (
	RepositorySelectionAll      RepositorySelection = "all"
	RepositorySelectionSelected RepositorySelection = "selected"
)

// String returns the string representation
func (r RepositorySelection) String() string {
	return string(r)
}

// IsAll returns true if all repositories are selected
func (r RepositorySelection) IsAll() bool {
	return r == RepositorySelectionAll
}

// IsSelected returns true if specific repositories are selected
func (r RepositorySelection) IsSelected() bool {
	return r == RepositorySelectionSelected
}

// WebhookEventType represents types of webhook events
type WebhookEventType string

const (
	WebhookEventPush                WebhookEventType = "push"
	WebhookEventPullRequest         WebhookEventType = "pull_request"
	WebhookEventInstallation        WebhookEventType = "installation"
	WebhookEventInstallationRepos   WebhookEventType = "installation_repositories"
	WebhookEventRepositoriesAdded   WebhookEventType = "repositories_added"
	WebhookEventRepositoriesRemoved WebhookEventType = "repositories_removed"
	WebhookEventInstallationCreated WebhookEventType = "created"
	WebhookEventInstallationDeleted WebhookEventType = "deleted"
)

// String returns the string representation
func (w WebhookEventType) String() string {
	return string(w)
}
