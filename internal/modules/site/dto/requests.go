package dto

import "github.com/kkz6/launch-go/internal/modules/site/enums"

// CreateSiteRequest represents the request to create a new site
type CreateSiteRequest struct {
	Address                     string         `json:"address" validate:"required,max=255"`
	Aliases                     []string       `json:"aliases" validate:"omitempty,dive,max=255"`
	PhpVersion                  string         `json:"php_version" validate:"required"`
	Type                        enums.SiteType `json:"type" validate:"required,oneof=laravel wordpress static generic"`
	WebFolder                   string   `json:"web_folder" validate:"omitempty,max=255"`
	ZeroDowntimeDeployment      bool     `json:"zero_downtime_deployment"`
	SourceControlID             *string  `json:"source_control_id" validate:"omitempty,ulid"`
	SourceControlRepositoriesID *string  `json:"source_control_repositories_id" validate:"omitempty"`
	RepositoryBranch            string   `json:"repository_branch" validate:"omitempty,max=255"`
	CreateDNSRecord             bool     `json:"create_dns_record"`
	ConnectedDomainID           *string  `json:"connected_domain_id" validate:"omitempty,ulid"`
	CreateDatabase              bool     `json:"create_database"`
	DatabaseOption              string   `json:"database_option" validate:"omitempty,oneof=new existing"`
	DatabaseID                  *string  `json:"database_id" validate:"omitempty,ulid"`
	DatabaseName                *string  `json:"database_name" validate:"omitempty,max=255"`
	DatabaseUserOption          string   `json:"database_user_option" validate:"omitempty,oneof=new existing"`
	DatabaseUserID              *string  `json:"database_user_id" validate:"omitempty,ulid"`
	DatabaseUserName            *string  `json:"database_user_name" validate:"omitempty,max=255"`
	DatabaseUserPassword        *string  `json:"database_user_password" validate:"omitempty,max=255"`
	CreateScheduler             bool     `json:"create_scheduler"`
	CreateQueue                 bool     `json:"create_queue"`
}

// UpdateSiteRequest represents the request to update a site
type UpdateSiteRequest struct {
	PhpVersion                   *string `json:"php_version" validate:"omitempty,oneof=php80 php81 php82 php83 php84"`
	WebFolder                    *string `json:"web_folder" validate:"omitempty,max=255"`
	RepositoryBranch             *string `json:"repository_branch" validate:"omitempty,max=255"`
	TlsSetting                   *string `json:"tls_setting" validate:"omitempty,oneof=auto custom internal off"`
	PrivateKey                   *string `json:"private_key" validate:"omitempty"`
	Certificate                  *string `json:"certificate" validate:"omitempty"`
	DeployNotificationEmail      *string `json:"deploy_notification_email" validate:"omitempty,email"`
	SharedDirectories            *string `json:"shared_directories" validate:"omitempty"`
	SharedFiles                  *string `json:"shared_files" validate:"omitempty"`
	WriteableDirectories         *string `json:"writeable_directories" validate:"omitempty"`
	HookBeforeUpdatingRepository *string `json:"hook_before_updating_repository" validate:"omitempty"`
	HookAfterUpdatingRepository  *string `json:"hook_after_updating_repository" validate:"omitempty"`
	HookBeforeMakingCurrent      *string `json:"hook_before_making_current" validate:"omitempty"`
	HookAfterMakingCurrent       *string `json:"hook_after_making_current" validate:"omitempty"`
	DeploymentReleasesRetention  *int    `json:"deployment_releases_retention" validate:"omitempty,min=1,max=50"`
	QueueDeployments             *bool   `json:"queue_deployments" validate:"omitempty"`
}

// UpdateSSLRequest represents the request to update SSL settings
type UpdateSSLRequest struct {
	TlsSetting  string  `json:"tls_setting" validate:"required,oneof=auto custom internal off"`
	PrivateKey  *string `json:"private_key" validate:"omitempty"`
	Certificate *string `json:"certificate" validate:"omitempty"`
}

// UpdateDeploymentSettingsRequest represents deployment settings update
type UpdateDeploymentSettingsRequest struct {
	DeployNotificationEmail      *string `json:"deploy_notification_email" validate:"omitempty,email"`
	SharedDirectories            *string `json:"shared_directories" validate:"omitempty"`
	SharedFiles                  *string `json:"shared_files" validate:"omitempty"`
	WriteableDirectories         *string `json:"writeable_directories" validate:"omitempty"`
	HookBeforeUpdatingRepository *string `json:"hook_before_updating_repository" validate:"omitempty"`
	HookAfterUpdatingRepository  *string `json:"hook_after_updating_repository" validate:"omitempty"`
	HookBeforeMakingCurrent      *string `json:"hook_before_making_current" validate:"omitempty"`
	HookAfterMakingCurrent       *string `json:"hook_after_making_current" validate:"omitempty"`
	DeploymentReleasesRetention  *int    `json:"deployment_releases_retention" validate:"omitempty,min=1,max=50"`
	QueueDeployments             *bool   `json:"queue_deployments" validate:"omitempty"`
}

// CreateQueueRequest represents the request to create a queue worker
type CreateQueueRequest struct {
	QueueConnection       string  `json:"queue_connection" validate:"required,max=50"`
	Queue                 string  `json:"queue" validate:"required,max=100"`
	User                  *string `json:"user" validate:"omitempty,max=255"`
	RestSecondsOnEmpty    int     `json:"rest_seconds_on_empty" validate:"required,min=0"`
	MaxSecondsPerJob      int     `json:"max_seconds_per_job" validate:"required,min=1"`
	FailedJobDelaySeconds int     `json:"failed_job_delay_seconds" validate:"required,min=0"`
	MaxTries              *int    `json:"max_tries" validate:"omitempty,min=0"`
	MaxMemory             *int    `json:"max_memory" validate:"omitempty,min=128"`
	RunOnMaintenance      bool    `json:"run_on_maintenance"`
	RunWithListen         bool    `json:"run_with_listen"`
	Directory             *string `json:"directory" validate:"omitempty,max=500"`
	Environment           *string `json:"environment" validate:"omitempty,max=100"`
	NumProcs              *int    `json:"numprocs" validate:"omitempty,min=1"`
	StopWaitSeconds       *int    `json:"stop_wait_seconds" validate:"omitempty,min=0"`
}

// CreateCommandRequest represents the request to run a command
type CreateCommandRequest struct {
	Command string `json:"command" validate:"required"`
}

// CreateRedirectRequest represents the request to create a redirect
type CreateRedirectRequest struct {
	From string `json:"from" validate:"required,max=500"`
	To   string `json:"to" validate:"required,max=500"`
	Mode int    `json:"mode" validate:"required,oneof=1 2"`
}

// RollbackRequest represents a rollback request
type RollbackRequest struct {
	DeploymentID string `json:"deployment_id" validate:"required,ulid"`
}

// UpdateAutoRestartQueueRequest represents the request to update auto-restart queue setting
type UpdateAutoRestartQueueRequest struct {
	Enabled bool `json:"enabled"`
}
