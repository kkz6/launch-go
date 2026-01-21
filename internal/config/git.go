package config

// GitConfig holds git provider configuration
type GitConfig struct {
	GitHub    GitHubConfig
	GitLab    GitLabConfig
	Bitbucket BitbucketConfig
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	AppID         string `env:"GITHUB_APP_ID" default:""`
	PrivateKey    string `env:"GITHUB_PRIVATE_KEY" default:""`
	WebhookSecret string `env:"GITHUB_WEBHOOK_SECRET" default:""`
	AppSlug       string `env:"GITHUB_APP_SLUG" default:""`
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	ClientID      string `env:"GITLAB_CLIENT_ID" default:""`
	ClientSecret  string `env:"GITLAB_CLIENT_SECRET" default:""`
	WebhookSecret string `env:"GITLAB_WEBHOOK_SECRET" default:""`
}

// BitbucketConfig holds Bitbucket-specific configuration
type BitbucketConfig struct {
	ClientID      string `env:"BITBUCKET_CLIENT_ID" default:""`
	ClientSecret  string `env:"BITBUCKET_CLIENT_SECRET" default:""`
	WebhookSecret string `env:"BITBUCKET_WEBHOOK_SECRET" default:""`
}
