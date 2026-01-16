package dto

import (
	"encoding/json"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// SiteResponse represents a site in API responses
type SiteResponse struct {
	ID                           string              `json:"id"`
	ServerID                     string              `json:"server_id"`
	UserID                       string              `json:"user_id"`
	Address                      string              `json:"address"`
	Type                         string              `json:"type"`
	Aliases                      []string            `json:"aliases,omitempty"`
	TlsSetting                   string              `json:"tls_setting"`
	ZeroDowntimeDeployment       bool                `json:"zero_downtime_deployment"`
	DeploymentReleasesRetention  int                 `json:"deployment_releases_retention"`
	RepositoryBranch             string              `json:"repository_branch"`
	DeployNotificationEmail      *string             `json:"deploy_notification_email,omitempty"`
	Path                         string              `json:"path"`
	WebFolder                    string              `json:"web_folder"`
	PhpVersion                   string              `json:"php_version"`
	AutoDeployment               bool                `json:"auto_deployment"`
	QueueDeployments             bool                `json:"queue_deployments"`
	AutoRestartQueue             bool                `json:"auto_restart_queue"`
	SharedDirectories            []string            `json:"shared_directories,omitempty"`
	WriteableDirectories         []string            `json:"writeable_directories,omitempty"`
	SharedFiles                  []string            `json:"shared_files,omitempty"`
	HookBeforeUpdatingRepository *string             `json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository  *string             `json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent      *string             `json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent       *string             `json:"hook_after_making_current,omitempty"`
	DeployToken                  *string             `json:"deploy_token,omitempty"`
	DeployWebhookURL              string              `json:"deploy_webhook_url,omitempty"`
	URL                           string              `json:"url"`
	ApplicationDirectory          string              `json:"app_directory"`
	RepositoryURL                 *string             `json:"repository_url,omitempty"`
	Status                        string              `json:"status"`
	InstalledAt                   *string             `json:"installed_at"`
	InstallationFailedAt          *string             `json:"installation_failed_at"`
	UninstallationRequestedAt     *string             `json:"uninstallation_requested_at"`
	UninstallationFailedAt        *string             `json:"uninstallation_failed_at"`
	LatestDeployment              *DeploymentResponse              `json:"latest_deployment,omitempty"`
	SourceControl                *SourceControlResponse           `json:"source_control,omitempty"`
	Repository                   *SourceControlRepositoryResponse `json:"repository,omitempty"`
	CreatedAt                    string                           `json:"created_at"`
	UpdatedAt                    string                           `json:"updated_at"`
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
	ID                    string                 `json:"id"`
	SiteID                string                 `json:"site_id"`
	ServerID              string                 `json:"server_id"`
	Name                  string                 `json:"name"`
	Directory             string                 `json:"directory"`
	Command               string                 `json:"command"`
	User                  string                 `json:"user"`
	QueueConnection       string                 `json:"queue_connection"`
	Queue                 string                 `json:"queue"`
	NumProcs              int                    `json:"numprocs"`
	MaxSecondsPerJob      int                    `json:"max_seconds_per_job"`
	MaxTries              int                    `json:"max_tries"`
	RestSecondsOnEmpty    int                    `json:"rest_seconds_on_empty"`
	FailedJobDelaySeconds int                    `json:"failed_job_delay_seconds"`
	MaxMemory             int                    `json:"max_memory"`
	RunOnMaintenance      bool                   `json:"run_on_maintenance"`
	RunWithListen         bool                   `json:"run_with_listen"`
	Running               bool                   `json:"running"`
	Info                  map[string]interface{} `json:"info,omitempty"`
	LastStatusCheck       *string                `json:"last_status_check,omitempty"`
	InstalledAt           *string                `json:"installed_at,omitempty"`
	CreatedAt             string                 `json:"created_at"`
}

// UserSummaryResponse represents limited user info in API responses
type UserSummaryResponse struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Photo *string `json:"photo,omitempty"`
}

// CommandResponse represents a command in API responses
type CommandResponse struct {
	ID        string               `json:"id"`
	SiteID    string               `json:"site_id"`
	UserID    string               `json:"user_id"`
	User      *UserSummaryResponse `json:"user,omitempty"`
	Command   string               `json:"command"`
	Status    string               `json:"status"`
	Output    *string              `json:"output,omitempty"`
	ExitCode  *int                 `json:"exit_code,omitempty"`
	CreatedAt string               `json:"created_at"`
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

// DeletionSummaryResponse represents resources that will be deleted with a site
type DeletionSummaryResponse struct {
	Queues int `json:"queues"`
	Crons  int `json:"crons"`
}

// TlsOptionResponse represents a TLS option for the settings page
type TlsOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PhpVersionResponse represents an installed PHP version
type PhpVersionResponse struct {
	Version   string `json:"version"`
	IsDefault bool   `json:"is_default"`
}

// SourceControlResponse represents source control info for a site
type SourceControlResponse struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	Login    *string `json:"login,omitempty"`
	Name     *string `json:"name,omitempty"`
	Type     *string `json:"type,omitempty"`
}

// SourceControlRepositoryResponse represents a linked repository
type SourceControlRepositoryResponse struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	FullName      string  `json:"full_name"`
	DefaultBranch string  `json:"default_branch"`
	HTMLURL       *string `json:"html_url,omitempty"`
}

// SiteSettingsResponse represents the site settings page data
type SiteSettingsResponse struct {
	Site              SiteResponse                     `json:"site"`
	TlsOptions        []TlsOptionResponse              `json:"tls_options"`
	PhpVersions       []PhpVersionResponse             `json:"php_versions"`
	ActiveCertificate *CertificateResponse             `json:"active_certificate,omitempty"`
	SourceControl     *SourceControlResponse           `json:"source_control,omitempty"`
	Repository        *SourceControlRepositoryResponse `json:"repository,omitempty"`
}

// ToSiteResponse converts a Site model to a response DTO
func ToSiteResponse(site *models.Site) SiteResponse {
	repositoryBranch := ""
	if site.RepositoryBranch != nil {
		repositoryBranch = *site.RepositoryBranch
	}

	phpVersion := ""
	if site.PhpVersion != nil {
		phpVersion = site.PhpVersion.String()
	}

	createdAt := ""
	if site.CreatedAt != nil {
		createdAt = site.CreatedAt.Format(time.RFC3339)
	}

	updatedAt := ""
	if site.UpdatedAt != nil {
		updatedAt = site.UpdatedAt.Format(time.RFC3339)
	}

	// Build deploy webhook URL if token exists
	var deployWebhookURL string
	if site.DeployToken != nil && *site.DeployToken != "" {
		deployWebhookURL = "/deploy/" + site.ID + "/" + *site.DeployToken
	}

	resp := SiteResponse{
		ID:                           site.ID,
		ServerID:                     site.ServerID,
		UserID:                       site.UserID,
		Address:                      site.Address,
		Type:                         string(site.Type),
		Aliases:                      site.Aliases,
		TlsSetting:                   string(site.TlsSetting),
		ZeroDowntimeDeployment:       site.ZeroDowntimeDeployment,
		DeploymentReleasesRetention:  site.DeploymentReleasesRetention,
		RepositoryBranch:             repositoryBranch,
		DeployNotificationEmail:      site.DeployNotificationEmail,
		Path:                         site.Path,
		WebFolder:                    site.WebFolder,
		PhpVersion:                   phpVersion,
		AutoDeployment:               site.AutoDeployment,
		QueueDeployments:             site.QueueDeployments,
		AutoRestartQueue:             site.AutoRestartQueue,
		SharedDirectories:            site.SharedDirectories,
		WriteableDirectories:         site.WriteableDirectories,
		SharedFiles:                  site.SharedFiles,
		HookBeforeUpdatingRepository: site.HookBeforeUpdatingRepository,
		HookAfterUpdatingRepository:  site.HookAfterUpdatingRepository,
		HookBeforeMakingCurrent:      site.HookBeforeMakingCurrent,
		HookAfterMakingCurrent:       site.HookAfterMakingCurrent,
		DeployToken:                  site.DeployToken,
		DeployWebhookURL:             deployWebhookURL,
		URL:                          site.GetURL(),
		ApplicationDirectory:         site.GetApplicationDirectory(),
		CreatedAt:                    createdAt,
		UpdatedAt:                    updatedAt,
	}

	// Set installation status
	resp.Status = string(site.Status())

	if site.InstalledAt != nil {
		installed := site.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &installed
	}

	if site.InstallationFailedAt != nil {
		failed := site.InstallationFailedAt.Format(time.RFC3339)
		resp.InstallationFailedAt = &failed
	}

	if site.UninstallationRequestedAt != nil {
		uninstallReq := site.UninstallationRequestedAt.Format(time.RFC3339)
		resp.UninstallationRequestedAt = &uninstallReq
	}

	if site.UninstallationFailedAt != nil {
		uninstallFailed := site.UninstallationFailedAt.Format(time.RFC3339)
		resp.UninstallationFailedAt = &uninstallFailed
	}

	if site.LatestDeployment != nil {
		deploymentResp := ToDeploymentResponse(site.LatestDeployment)
		resp.LatestDeployment = &deploymentResp
	}

	return resp
}

// ToDeploymentResponse converts a Deployment model to a response DTO
func ToDeploymentResponse(deployment *models.Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:           deployment.ID,
		SiteID:       deployment.SiteID,
		UserID:       deployment.UserID,
		TaskID:       deployment.TaskID,
		Status:       string(deployment.Status),
		GitHash:      deployment.GitHash,
		ShortGitHash: deployment.GetShortGitHash(),
		CommitData:   deployment.CommitData,
		IsRollback:   deployment.IsRollback(),
		CreatedAt:    deployment.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    deployment.UpdatedAt.Format(time.RFC3339),
	}
}

// ToCertificateResponse converts a Certificate model to a response DTO
func ToCertificateResponse(cert *models.Certificate) CertificateResponse {
	resp := CertificateResponse{
		ID:        cert.ID,
		SiteID:    cert.SiteID,
		Type:      string(cert.Type),
		Domains:   cert.Domains,
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
func ToQueueResponse(queue *models.Queue) QueueResponse {
	directory := ""
	if queue.Directory != nil {
		directory = *queue.Directory
	}

	var maxSecondsPerJob, maxTries, restSecondsOnEmpty, failedJobDelaySeconds, maxMemory int
	if queue.MaxSecondsPerJob != nil {
		maxSecondsPerJob = *queue.MaxSecondsPerJob
	}
	if queue.MaxTries != nil {
		maxTries = *queue.MaxTries
	}
	if queue.RestSecondsOnEmpty != nil {
		restSecondsOnEmpty = *queue.RestSecondsOnEmpty
	}
	if queue.FailedJobDelaySeconds != nil {
		failedJobDelaySeconds = *queue.FailedJobDelaySeconds
	}
	if queue.MaxMemory != nil {
		maxMemory = *queue.MaxMemory
	}

	createdAt := ""
	if queue.CreatedAt != nil {
		createdAt = queue.CreatedAt.Format(time.RFC3339)
	}

	resp := QueueResponse{
		ID:                    queue.ID,
		SiteID:                queue.SiteID,
		ServerID:              queue.ServerID,
		Name:                  queue.QueueName,
		Directory:             directory,
		Command:               queue.BuildCommand(),
		User:                  queue.User,
		QueueConnection:       queue.QueueConnection,
		Queue:                 queue.QueueName,
		NumProcs:              queue.NumProcs,
		MaxSecondsPerJob:      maxSecondsPerJob,
		MaxTries:              maxTries,
		RestSecondsOnEmpty:    restSecondsOnEmpty,
		FailedJobDelaySeconds: failedJobDelaySeconds,
		MaxMemory:             maxMemory,
		RunOnMaintenance:      queue.RunOnMaintenance,
		RunWithListen:         queue.RunWithListen,
		Running:               queue.Running,
		CreatedAt:             createdAt,
	}

	// Parse info JSON if present
	if queue.Info != nil && *queue.Info != "" {
		var info map[string]interface{}
		if err := json.Unmarshal([]byte(*queue.Info), &info); err == nil {
			resp.Info = info
		}
	}

	if queue.LastStatusCheck != nil {
		lastCheck := queue.LastStatusCheck.Format(time.RFC3339)
		resp.LastStatusCheck = &lastCheck
	}

	if queue.InstalledAt != nil {
		installed := queue.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &installed
	}

	return resp
}

// ToCommandResponse converts a Command model to a response DTO
func ToCommandResponse(cmd *models.Command) CommandResponse {
	createdAt := ""
	if cmd.CreatedAt != nil {
		createdAt = cmd.CreatedAt.Format(time.RFC3339)
	}

	resp := CommandResponse{
		ID:        cmd.ID,
		SiteID:    cmd.SiteID,
		UserID:    cmd.UserID,
		Command:   cmd.Command,
		Status:    string(cmd.Status),
		Output:    cmd.Output,
		ExitCode:  cmd.ExitCode,
		CreatedAt: createdAt,
	}

	// Include user details if loaded
	if cmd.User != nil {
		resp.User = &UserSummaryResponse{
			ID:    cmd.User.ID,
			Name:  cmd.User.Name,
			Email: cmd.User.Email,
			Photo: cmd.User.ProfilePhotoPath,
		}
	}

	return resp
}

// ToRedirectResponse converts a Redirect model to a response DTO
func ToRedirectResponse(redirect *models.Redirect) RedirectResponse {
	modeLabel := "Temporary (302)"
	if redirect.Mode == 301 {
		modeLabel = "Permanent (301)"
	}

	createdAt := ""
	if redirect.CreatedAt != nil {
		createdAt = redirect.CreatedAt.Format(time.RFC3339)
	}

	return RedirectResponse{
		ID:        redirect.ID,
		SiteID:    redirect.SiteID,
		Mode:      redirect.Mode,
		ModeLabel: modeLabel,
		From:      redirect.From,
		To:        redirect.To,
		Status:    redirect.Status,
		CreatedAt: createdAt,
	}
}

// VerifyDomainResponse represents the domain verification result
type VerifyDomainResponse struct {
	Verified          bool    `json:"verified"`
	Domain            string  `json:"domain"`
	BaseDomain        string  `json:"base_domain"`
	ConnectedDomainID *string `json:"connected_domain_id,omitempty"`
	CanCreateRecord   bool    `json:"can_create_record"`
}
