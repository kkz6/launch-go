package providers

import (
	"context"
	"errors"
)

var (
	ErrInvalidSignature       = errors.New("invalid webhook signature")
	ErrInvalidPayload         = errors.New("invalid webhook payload")
	ErrProviderNotConfigured  = errors.New("provider not configured")
	ErrInstallationNotFound   = errors.New("installation not found")
	ErrRepositoryNotFound     = errors.New("repository not found")
	ErrPermissionDenied       = errors.New("permission denied")
	ErrAuthenticationFailed   = errors.New("authentication failed")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
)

// Provider defines the interface for git provider integrations
type Provider interface {
	// GetType returns the provider type
	GetType() GitProviderType

	// GetInstallationURL returns the URL to install the app
	GetInstallationURL() (string, error)

	// GetInstallation gets an installation by ID
	GetInstallation(ctx context.Context, installationID string) (*AppInstallationData, error)

	// GetAllInstallations gets all installations for the app
	GetAllInstallations(ctx context.Context) ([]AppInstallationData, error)

	// GetInstallationRepositories gets repositories for an installation
	GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error)

	// GetRepository gets a specific repository
	GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error)

	// ValidateWebhook validates a webhook signature
	ValidateWebhook(payload []byte, signature string) bool

	// GetCommitData extracts commit data from a webhook payload
	GetCommitData(payload map[string]interface{}) *CommitData

	// TestConnection tests the connection to the provider
	TestConnection(ctx context.Context) error

	// GetSSHURL returns the SSH URL for a repository
	GetSSHURL(repo string) string

	// GetHTTPSURL returns the HTTPS URL for a repository
	GetHTTPSURL(repo string) string

	// DeployKey deploys an SSH key to a repository
	DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error

	// GetLastCommit gets the last commit for a repository and branch
	GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*CommitData, error)

	// GetInstallationToken gets a temporary access token for an installation
	// Used for HTTPS-based git cloning during deployments
	GetInstallationToken(ctx context.Context, installationID string) (string, error)

	// CreateDeployment creates a deployment on the git provider
	// This is used to show deployment status in the provider's UI (e.g., GitHub Deployments)
	CreateDeployment(ctx context.Context, info *DeploymentInfo) (*DeploymentResult, error)

	// UpdateDeploymentStatus updates the status of a deployment on the git provider
	UpdateDeploymentStatus(ctx context.Context, info *DeploymentInfo, vcsData map[string]interface{}, status DeploymentStatus) error
}

// ProviderConfig holds configuration for a git provider
type ProviderConfig struct {
	AppID         string
	PrivateKey    string
	WebhookSecret string
	AppSlug       string
	// ClientID and ClientSecret are only used for GitLab/Bitbucket OAuth
	ClientID     string
	ClientSecret string
}

// ProviderFactory creates providers based on type
type ProviderFactory struct {
	configs map[GitProviderType]*ProviderConfig
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		configs: make(map[GitProviderType]*ProviderConfig),
	}
}

// RegisterConfig registers a configuration for a provider type
func (f *ProviderFactory) RegisterConfig(providerType GitProviderType, config *ProviderConfig) {
	f.configs[providerType] = config
}

// GetProvider creates a provider instance for the given type
func (f *ProviderFactory) GetProvider(providerType GitProviderType) (Provider, error) {
	config, ok := f.configs[providerType]
	if !ok {
		return nil, ErrProviderNotConfigured
	}

	switch providerType {
	case GitProviderGitHub:
		return NewGitHubProvider(config), nil
	case GitProviderGitLab:
		return NewGitLabProvider(config), nil
	case GitProviderBitbucket:
		return NewBitbucketProvider(config), nil
	default:
		return nil, ErrProviderNotConfigured
	}
}

// GetProviderWithInstallation creates a provider instance with source control context
func (f *ProviderFactory) GetProviderWithInstallation(
	providerType GitProviderType,
	sourceControl *SourceControlData,
) (Provider, error) {
	provider, err := f.GetProvider(providerType)
	if err != nil {
		return nil, err
	}

	// Set source control context if the provider supports it
	if contextProvider, ok := provider.(SourceControlContextProvider); ok {
		contextProvider.SetSourceControl(sourceControl)
	}

	return provider, nil
}

// SourceControlContextProvider is an interface for providers that can use source control context
type SourceControlContextProvider interface {
	SetSourceControl(sc *SourceControlData)
}
