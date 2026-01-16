package config

import (
	"github.com/spf13/viper"
)

// GitConfig holds git provider configuration
type GitConfig struct {
	GitHub    GitHubConfig
	GitLab    GitLabConfig
	Bitbucket BitbucketConfig
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	AppID         string
	PrivateKey    string
	WebhookSecret string
	AppSlug       string
}

// GitLabConfig holds GitLab-specific configuration
type GitLabConfig struct {
	ClientID      string
	ClientSecret  string
	WebhookSecret string
}

// BitbucketConfig holds Bitbucket-specific configuration
type BitbucketConfig struct {
	ClientID      string
	ClientSecret  string
	WebhookSecret string
}

func loadGitConfig() GitConfig {
	return GitConfig{
		GitHub: GitHubConfig{
			AppID:         viper.GetString("GITHUB_APP_ID"),
			PrivateKey:    viper.GetString("GITHUB_PRIVATE_KEY"),
			WebhookSecret: viper.GetString("GITHUB_WEBHOOK_SECRET"),
			AppSlug:       viper.GetString("GITHUB_APP_SLUG"),
		},
		GitLab: GitLabConfig{
			ClientID:      viper.GetString("GITLAB_CLIENT_ID"),
			ClientSecret:  viper.GetString("GITLAB_CLIENT_SECRET"),
			WebhookSecret: viper.GetString("GITLAB_WEBHOOK_SECRET"),
		},
		Bitbucket: BitbucketConfig{
			ClientID:      viper.GetString("BITBUCKET_CLIENT_ID"),
			ClientSecret:  viper.GetString("BITBUCKET_CLIENT_SECRET"),
			WebhookSecret: viper.GetString("BITBUCKET_WEBHOOK_SECRET"),
		},
	}
}

func setGitDefaults() {
	viper.SetDefault("GITHUB_APP_ID", "")
	viper.SetDefault("GITHUB_PRIVATE_KEY", "")
	viper.SetDefault("GITHUB_WEBHOOK_SECRET", "")
	viper.SetDefault("GITHUB_APP_SLUG", "")
	viper.SetDefault("GITLAB_CLIENT_ID", "")
	viper.SetDefault("GITLAB_CLIENT_SECRET", "")
	viper.SetDefault("GITLAB_WEBHOOK_SECRET", "")
	viper.SetDefault("BITBUCKET_CLIENT_ID", "")
	viper.SetDefault("BITBUCKET_CLIENT_SECRET", "")
	viper.SetDefault("BITBUCKET_WEBHOOK_SECRET", "")
}
