package jobs

// Job type constants
const (
	TypeProcessGitWebhook         = "git:process_webhook"
	TypeSyncInstallationRepos     = "git:sync_installation_repos"
	TypeTriggerDeploymentFromPush = "git:trigger_deployment"
)

// ProcessGitWebhookPayload is the payload for processing git webhooks
type ProcessGitWebhookPayload struct {
	Provider  string `json:"provider"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// SyncInstallationReposPayload is the payload for syncing installation repositories
type SyncInstallationReposPayload struct {
	Provider       string `json:"provider"`
	InstallationID string `json:"installation_id"`
	TeamID         string `json:"team_id"`
	UserID         string `json:"user_id"`
}

// TriggerDeploymentPayload is the payload for triggering deployments from git push
type TriggerDeploymentPayload struct {
	Provider    string                 `json:"provider"`
	Repository  string                 `json:"repository"`
	Branch      string                 `json:"branch"`
	WebhookData map[string]interface{} `json:"webhook_data"`
}
