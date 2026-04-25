// Package types contains all type definitions for the git module
package types

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/pkg/enumtypes"
)

// =============================================================================
// GitProviderType
// =============================================================================

// GitProviderType represents the type of git provider
type GitProviderType string

const (
	GitProviderGitHub    GitProviderType = "github"
	GitProviderGitLab    GitProviderType = "gitlab"
	GitProviderBitbucket GitProviderType = "bitbucket"
)

var allGitProviders = []GitProviderType{
	GitProviderGitHub,
	GitProviderGitLab,
	GitProviderBitbucket,
}

// AllGitProviders returns all available git providers
func AllGitProviders() []GitProviderType {
	return allGitProviders
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
		s := string(p)
		if len(s) == 0 {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
}

// IsValid checks if the provider type is valid
func (p GitProviderType) IsValid() bool {
	return enumtypes.IsValid(p, allGitProviders...)
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
	return enumtypes.Value(p)
}

// Scan implements sql.Scanner for database retrieval
func (p *GitProviderType) Scan(value any) error {
	return enumtypes.Scan(p, value)
}

// =============================================================================
// AccountType
// =============================================================================

// AccountType represents the type of git account (user or organization)
type AccountType string

const (
	AccountTypeUser         AccountType = "user"
	AccountTypeOrganization AccountType = "organization"
)

var allAccountTypes = []AccountType{
	AccountTypeUser,
	AccountTypeOrganization,
}

var accountTypeLabels = map[AccountType]string{
	AccountTypeUser:         "User",
	AccountTypeOrganization: "Organization",
}

// AllAccountTypes returns all valid account types
func AllAccountTypes() []AccountType {
	return allAccountTypes
}

// String returns the string representation of the account type
func (a AccountType) String() string {
	return string(a)
}

// Label returns a human-readable label for the account type
func (a AccountType) Label() string {
	return enumtypes.Label(a, accountTypeLabels, string(a))
}

// IsValid checks if the account type is valid
func (a AccountType) IsValid() bool {
	return enumtypes.IsValid(a, allAccountTypes...)
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

// Value implements driver.Valuer for database storage
func (a AccountType) Value() (driver.Value, error) {
	return enumtypes.Value(a)
}

// Scan implements sql.Scanner for database retrieval
func (a *AccountType) Scan(value any) error {
	return enumtypes.Scan(a, value)
}

// =============================================================================
// RepositorySelection
// =============================================================================

// RepositorySelection represents how repositories are selected for an installation
type RepositorySelection string

const (
	RepositorySelectionAll      RepositorySelection = "all"
	RepositorySelectionSelected RepositorySelection = "selected"
)

var allRepositorySelections = []RepositorySelection{
	RepositorySelectionAll,
	RepositorySelectionSelected,
}

var repositorySelectionLabels = map[RepositorySelection]string{
	RepositorySelectionAll:      "All",
	RepositorySelectionSelected: "Selected",
}

// AllRepositorySelections returns all valid repository selections
func AllRepositorySelections() []RepositorySelection {
	return allRepositorySelections
}

// String returns the string representation
func (r RepositorySelection) String() string {
	return string(r)
}

// Label returns a human-readable label
func (r RepositorySelection) Label() string {
	return enumtypes.Label(r, repositorySelectionLabels, string(r))
}

// IsValid checks if the selection is valid
func (r RepositorySelection) IsValid() bool {
	return enumtypes.IsValid(r, allRepositorySelections...)
}

// IsAll returns true if all repositories are selected
func (r RepositorySelection) IsAll() bool {
	return r == RepositorySelectionAll
}

// IsSelected returns true if specific repositories are selected
func (r RepositorySelection) IsSelected() bool {
	return r == RepositorySelectionSelected
}

// Value implements driver.Valuer for database storage
func (r RepositorySelection) Value() (driver.Value, error) {
	return enumtypes.Value(r)
}

// Scan implements sql.Scanner for database retrieval
func (r *RepositorySelection) Scan(value any) error {
	return enumtypes.Scan(r, value)
}

// =============================================================================
// WebhookEventType
// =============================================================================

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

var allWebhookEventTypes = []WebhookEventType{
	WebhookEventPush,
	WebhookEventPullRequest,
	WebhookEventInstallation,
	WebhookEventInstallationRepos,
	WebhookEventRepositoriesAdded,
	WebhookEventRepositoriesRemoved,
	WebhookEventInstallationCreated,
	WebhookEventInstallationDeleted,
}

var webhookEventTypeLabels = map[WebhookEventType]string{
	WebhookEventPush:                "Push",
	WebhookEventPullRequest:         "Pull Request",
	WebhookEventInstallation:        "Installation",
	WebhookEventInstallationRepos:   "Installation Repositories",
	WebhookEventRepositoriesAdded:   "Repositories Added",
	WebhookEventRepositoriesRemoved: "Repositories Removed",
	WebhookEventInstallationCreated: "Installation Created",
	WebhookEventInstallationDeleted: "Installation Deleted",
}

// AllWebhookEventTypes returns all valid webhook event types
func AllWebhookEventTypes() []WebhookEventType {
	return allWebhookEventTypes
}

// String returns the string representation
func (w WebhookEventType) String() string {
	return string(w)
}

// Label returns a human-readable label
func (w WebhookEventType) Label() string {
	return enumtypes.Label(w, webhookEventTypeLabels, string(w))
}

// IsValid checks if the event type is valid
func (w WebhookEventType) IsValid() bool {
	return enumtypes.IsValid(w, allWebhookEventTypes...)
}

// Value implements driver.Valuer for database storage
func (w WebhookEventType) Value() (driver.Value, error) {
	return enumtypes.Value(w)
}

// Scan implements sql.Scanner for database retrieval
func (w *WebhookEventType) Scan(value any) error {
	return enumtypes.Scan(w, value)
}
