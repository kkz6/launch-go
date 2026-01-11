package site

import "time"

// CreateSiteRequest represents the request to create a new site
type CreateSiteRequest struct {
	Address                     string   `json:"address" validate:"required,max=255"`
	Aliases                     []string `json:"aliases" validate:"omitempty,dive,max=255"`
	PhpVersion                  string   `json:"php_version" validate:"required,oneof=8.1 8.2 8.3 8.4"`
	Type                        string   `json:"type" validate:"required,oneof=laravel wordpress static generic"`
	WebFolder                   string   `json:"web_folder" validate:"omitempty,max=255"`
	ZeroDowntimeDeployment      bool     `json:"zero_downtime_deployment"`
	SourceControlID             *string  `json:"source_control_id" validate:"omitempty,ulid"`
	SourceControlRepositoriesID *string  `json:"source_control_repositories_id" validate:"omitempty,ulid"`
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
	PhpVersion                   *string `json:"php_version" validate:"omitempty,oneof=8.1 8.2 8.3 8.4"`
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

// SiteResponse represents a site in API responses
type SiteResponse struct {
	ID                          string              `json:"id"`
	ServerID                    string              `json:"server_id"`
	UserID                      string              `json:"user_id"`
	Address                     string              `json:"address"`
	Type                        string              `json:"type"`
	Aliases                     []string            `json:"aliases,omitempty"`
	TlsSetting                  string              `json:"tls_setting"`
	ZeroDowntimeDeployment      bool                `json:"zero_downtime_deployment"`
	DeploymentReleasesRetention int                 `json:"deployment_releases_retention"`
	RepositoryBranch            string              `json:"repository_branch"`
	DeployNotificationEmail     *string             `json:"deploy_notification_email,omitempty"`
	Path                        string              `json:"path"`
	WebFolder                   string              `json:"web_folder"`
	PhpVersion                  string              `json:"php_version"`
	AutoDeployment              bool                `json:"auto_deployment"`
	QueueDeployments            bool                `json:"queue_deployments"`
	AutoRestartQueue            bool                `json:"auto_restart_queue"`
	SharedDirectories           []string            `json:"shared_directories,omitempty"`
	WriteableDirectories        []string            `json:"writeable_directories,omitempty"`
	SharedFiles                 []string            `json:"shared_files,omitempty"`
	URL                         string              `json:"url"`
	ApplicationDirectory        string              `json:"app_directory"`
	RepositoryURL               *string             `json:"repository_url,omitempty"`
	InstalledAt                 *string             `json:"installed_at,omitempty"`
	LatestDeployment            *DeploymentResponse `json:"latest_deployment,omitempty"`
	CreatedAt                   string              `json:"created_at"`
	UpdatedAt                   string              `json:"updated_at"`
}

// DeploymentResponse represents a deployment in API responses
type DeploymentResponse struct {
	ID           string                 `json:"id"`
	SiteID       string                 `json:"site_id"`
	UserID       *string                `json:"user_id,omitempty"`
	TaskID       *string                `json:"task_id,omitempty"`
	Status       string                 `json:"status"`
	GitHash      *string                `json:"git_hash,omitempty"`
	ShortGitHash string                 `json:"short_git_hash,omitempty"`
	CommitData   map[string]interface{} `json:"commit_data,omitempty"`
	IsRollback   bool                   `json:"is_rollback"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// CertificateResponse represents a certificate in API responses
type CertificateResponse struct {
	ID         string   `json:"id"`
	SiteID     string   `json:"site_id"`
	Type       string   `json:"type"`
	Domains    []string `json:"domains,omitempty"`
	IsActive   bool     `json:"is_active"`
	UploadedAt *string  `json:"uploaded_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

// QueueResponse represents a queue in API responses
type QueueResponse struct {
	ID                    string  `json:"id"`
	SiteID                string  `json:"site_id"`
	ServerID              string  `json:"server_id"`
	Name                  string  `json:"name"`
	Directory             string  `json:"directory"`
	Command               string  `json:"command"`
	User                  string  `json:"user"`
	QueueConnection       string  `json:"queue_connection"`
	Queue                 string  `json:"queue"`
	NumProcs              int     `json:"numprocs"`
	MaxSecondsPerJob      int     `json:"max_seconds_per_job"`
	MaxTries              int     `json:"max_tries"`
	RestSecondsOnEmpty    int     `json:"rest_seconds_on_empty"`
	FailedJobDelaySeconds int     `json:"failed_job_delay_seconds"`
	MaxMemory             int     `json:"max_memory"`
	RunOnMaintenance      bool    `json:"run_on_maintenance"`
	RunWithListen         bool    `json:"run_with_listen"`
	Running               bool    `json:"running"`
	InstalledAt           *string `json:"installed_at,omitempty"`
	CreatedAt             string  `json:"created_at"`
}

// CommandResponse represents a command in API responses
type CommandResponse struct {
	ID        string  `json:"id"`
	SiteID    string  `json:"site_id"`
	UserID    string  `json:"user_id"`
	Command   string  `json:"command"`
	Status    string  `json:"status"`
	Output    *string `json:"output,omitempty"`
	ExitCode  *int    `json:"exit_code,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// RedirectResponse represents a redirect in API responses
type RedirectResponse struct {
	ID        string `json:"id"`
	SiteID    string `json:"site_id"`
	Mode      int    `json:"mode"`
	ModeLabel string `json:"mode_label"`
	From      string `json:"from"`
	To        string `json:"to"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ReleaseResponse represents a release in API responses
type ReleaseResponse struct {
	ID         string  `json:"id"`
	SiteID     string  `json:"site_id"`
	Path       string  `json:"path"`
	CommitHash *string `json:"commit_hash,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// DeletionSummaryResponse represents resources that will be deleted with a site
type DeletionSummaryResponse struct {
	Queues int `json:"queues"`
	Crons  int `json:"crons"`
}

// ToSiteResponse converts a Site model to a response DTO
func ToSiteResponse(site *Site) SiteResponse {
	resp := SiteResponse{
		ID:                          site.ID,
		ServerID:                    site.ServerID,
		UserID:                      site.UserID,
		Address:                     site.Address,
		Type:                        string(site.Type),
		Aliases:                     site.GetAliases(),
		TlsSetting:                  string(site.TlsSetting),
		ZeroDowntimeDeployment:      site.ZeroDowntimeDeployment,
		DeploymentReleasesRetention: site.DeploymentReleasesRetention,
		RepositoryBranch:            site.RepositoryBranch,
		DeployNotificationEmail:     site.DeployNotificationEmail,
		Path:                        site.Path,
		WebFolder:                   site.WebFolder,
		PhpVersion:                  site.PhpVersion,
		AutoDeployment:              site.AutoDeployment,
		QueueDeployments:            site.QueueDeployments,
		AutoRestartQueue:            site.AutoRestartQueue,
		SharedDirectories:           site.GetSharedDirectories(),
		WriteableDirectories:        site.GetWriteableDirectories(),
		SharedFiles:                 site.GetSharedFiles(),
		URL:                         site.GetURL(),
		ApplicationDirectory:        site.GetApplicationDirectory(),
		CreatedAt:                   site.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                   site.UpdatedAt.Format(time.RFC3339),
	}

	if site.InstalledAt != nil {
		installed := site.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &installed
	}

	if site.LatestDeployment != nil {
		deploymentResp := ToDeploymentResponse(site.LatestDeployment)
		resp.LatestDeployment = &deploymentResp
	}

	return resp
}

// ToDeploymentResponse converts a Deployment model to a response DTO
func ToDeploymentResponse(deployment *Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:           deployment.ID,
		SiteID:       deployment.SiteID,
		UserID:       deployment.UserID,
		TaskID:       deployment.TaskID,
		Status:       string(deployment.Status),
		GitHash:      deployment.GitHash,
		ShortGitHash: deployment.GetShortGitHash(),
		CommitData:   deployment.GetCommitData(),
		IsRollback:   deployment.IsRollback(),
		CreatedAt:    deployment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    deployment.UpdatedAt.Format(time.RFC3339),
	}
}

// ToCertificateResponse converts a Certificate model to a response DTO
func ToCertificateResponse(cert *Certificate) CertificateResponse {
	resp := CertificateResponse{
		ID:        cert.ID,
		SiteID:    cert.SiteID,
		Type:      string(cert.Type),
		Domains:   cert.GetDomains(),
		IsActive:  cert.IsActive,
		CreatedAt: cert.CreatedAt.Format(time.RFC3339),
	}

	if cert.UploadedAt != nil {
		uploaded := cert.UploadedAt.Format(time.RFC3339)
		resp.UploadedAt = &uploaded
	}

	return resp
}

// ToQueueResponse converts a Queue model to a response DTO
func ToQueueResponse(queue *Queue) QueueResponse {
	resp := QueueResponse{
		ID:                    queue.ID,
		SiteID:                queue.SiteID,
		ServerID:              queue.ServerID,
		Name:                  queue.Name,
		Directory:             queue.Directory,
		Command:               queue.BuildCommand(),
		User:                  queue.User,
		QueueConnection:       queue.QueueConnection,
		Queue:                 queue.QueueName,
		NumProcs:              queue.NumProcs,
		MaxSecondsPerJob:      queue.MaxSecondsPerJob,
		MaxTries:              queue.MaxTries,
		RestSecondsOnEmpty:    queue.RestSecondsOnEmpty,
		FailedJobDelaySeconds: queue.FailedJobDelaySeconds,
		MaxMemory:             queue.MaxMemory,
		RunOnMaintenance:      queue.RunOnMaintenance,
		RunWithListen:         queue.RunWithListen,
		Running:               queue.Running,
		CreatedAt:             queue.CreatedAt.Format(time.RFC3339),
	}

	if queue.InstalledAt != nil {
		installed := queue.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &installed
	}

	return resp
}

// ToCommandResponse converts a Command model to a response DTO
func ToCommandResponse(cmd *Command) CommandResponse {
	return CommandResponse{
		ID:        cmd.ID,
		SiteID:    cmd.SiteID,
		UserID:    cmd.UserID,
		Command:   cmd.Command,
		Status:    string(cmd.Status),
		Output:    cmd.Output,
		ExitCode:  cmd.ExitCode,
		CreatedAt: cmd.CreatedAt.Format(time.RFC3339),
	}
}

// ToRedirectResponse converts a Redirect model to a response DTO
func ToRedirectResponse(redirect *Redirect) RedirectResponse {
	return RedirectResponse{
		ID:        redirect.ID,
		SiteID:    redirect.SiteID,
		Mode:      int(redirect.Mode),
		ModeLabel: redirect.Mode.Label(),
		From:      redirect.From,
		To:        redirect.To,
		Status:    redirect.Status,
		CreatedAt: redirect.CreatedAt.Format(time.RFC3339),
	}
}

// ToReleaseResponse converts a Release model to a response DTO
func ToReleaseResponse(release *Release) ReleaseResponse {
	return ReleaseResponse{
		ID:         release.ID,
		SiteID:     release.SiteID,
		Path:       release.Path,
		CommitHash: release.CommitHash,
		CreatedAt:  release.CreatedAt.Format(time.RFC3339),
	}
}
