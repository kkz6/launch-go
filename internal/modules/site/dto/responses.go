package dto

import (
	"encoding/json"

	"github.com/kkz6/launch-go/internal/modules/site/formatter"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// SiteResponse represents a site in API responses
type SiteResponse struct {
	ID                           string                           `json:"id"`
	ServerID                     string                           `json:"server_id"`
	UserID                       string                           `json:"user_id"`
	Address                      string                           `json:"address"`
	Type                         string                           `json:"type"`
	Aliases                      []string                         `json:"aliases,omitempty"`
	TLSSetting                   string                           `json:"tls_setting"`
	ZeroDowntimeDeployment       bool                             `json:"zero_downtime_deployment"`
	DeploymentReleasesRetention  int                              `json:"deployment_releases_retention"`
	RepositoryBranch             string                           `json:"repository_branch"`
	Path                         string                           `json:"path"`
	WebFolder                    string                           `json:"web_folder"`
	PhpVersion                   string                           `json:"php_version"`
	AutoDeployment               bool                             `json:"auto_deployment"`
	QueueDeployments             bool                             `json:"queue_deployments"`
	AutoRestartQueue             bool                             `json:"auto_restart_queue"`
	SharedDirectories            []string                         `json:"shared_directories,omitempty"`
	WriteableDirectories         []string                         `json:"writeable_directories,omitempty"`
	SharedFiles                  []string                         `json:"shared_files,omitempty"`
	HookBeforeUpdatingRepository *string                          `json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository  *string                          `json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent      *string                          `json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent       *string                          `json:"hook_after_making_current,omitempty"`
	DeployToken                  *string                          `json:"deploy_token,omitempty"`
	DeployWebhookURL             string                           `json:"deploy_webhook_url,omitempty"`
	URL                          string                           `json:"url"`
	ApplicationDirectory         string                           `json:"app_directory"`
	RepositoryURL                *string                          `json:"repository_url,omitempty"`
	EnabledFeatures              []string                         `json:"enabled_features,omitempty"`
	PendingFeatures              []string                         `json:"pending_features,omitempty"`
	LoadBalancedUpstreamID       *string                          `json:"load_balanced_upstream_id,omitempty"`
	Status                       string                           `json:"status"`
	InstalledAt                  *string                          `json:"installed_at"`
	InstallationFailedAt         *string                          `json:"installation_failed_at"`
	UninstallationRequestedAt    *string                          `json:"uninstallation_requested_at"`
	UninstallationFailedAt       *string                          `json:"uninstallation_failed_at"`
	LatestDeployment             *DeploymentResponse              `json:"latest_deployment,omitempty"`
	SourceControl                *SourceControlResponse           `json:"source_control,omitempty"`
	Repository                   *SourceControlRepositoryResponse `json:"repository,omitempty"`
	CreatedAt                    string                           `json:"created_at"`
	UpdatedAt                    string                           `json:"updated_at"`
}

// DeploymentResponse represents a deployment in API responses
type DeploymentResponse struct {
	ID           string         `json:"id"`
	SiteID       string         `json:"site_id"`
	UserID       *string        `json:"user_id,omitempty"`
	TaskID       *string        `json:"task_id,omitempty"`
	Status       string         `json:"status"`
	GitHash      *string        `json:"git_hash,omitempty"`
	ShortGitHash string         `json:"short_git_hash,omitempty"`
	CommitData   map[string]any `json:"commit_data,omitempty"`
	IsRollback   bool           `json:"is_rollback"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
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
	ID                    string         `json:"id"`
	SiteID                string         `json:"site_id"`
	ServerID              string         `json:"server_id"`
	Name                  string         `json:"name"`
	Directory             string         `json:"directory"`
	Command               string         `json:"command"`
	User                  string         `json:"user"`
	QueueConnection       string         `json:"queue_connection"`
	Queue                 string         `json:"queue"`
	NumProcs              int            `json:"numprocs"`
	MaxSecondsPerJob      int            `json:"max_seconds_per_job"`
	MaxTries              int            `json:"max_tries"`
	RestSecondsOnEmpty    int            `json:"rest_seconds_on_empty"`
	FailedJobDelaySeconds int            `json:"failed_job_delay_seconds"`
	MaxMemory             int            `json:"max_memory"`
	RunOnMaintenance      bool           `json:"run_on_maintenance"`
	RunWithListen         bool           `json:"run_with_listen"`
	Running               bool           `json:"running"`
	Info                  map[string]any `json:"info,omitempty"`
	LastStatusCheck       *string        `json:"last_status_check,omitempty"`
	InstalledAt           *string        `json:"installed_at,omitempty"`
	CreatedAt             string         `json:"created_at"`
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
	Type      int    `json:"type"`
	TypeLabel string `json:"type_label"`
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

// TLSOptionResponse represents a TLS option for the settings page
type TLSOptionResponse struct {
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
	TLSOptions        []TLSOptionResponse              `json:"tls_options"`
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

	// Build deploy webhook URL if token exists
	var deployWebhookURL string
	if site.DeployToken != nil && *site.DeployToken != "" {
		deployWebhookURL = "/deploy/" + site.ID + "/" + *site.DeployToken
	}

	// Extract enabled feature names
	var enabledFeatures []string
	for _, f := range site.EnabledFeatures {
		enabledFeatures = append(enabledFeatures, f.Name)
	}

	resp := SiteResponse{
		ID:                           site.ID,
		ServerID:                     site.ServerID,
		UserID:                       site.UserID,
		Address:                      site.Address,
		Type:                         string(site.Type),
		Aliases:                      site.Aliases,
		TLSSetting:                   string(site.TLSSetting),
		ZeroDowntimeDeployment:       site.ZeroDowntimeDeployment,
		DeploymentReleasesRetention:  site.DeploymentReleasesRetention,
		RepositoryBranch:             repositoryBranch,
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
		EnabledFeatures:              enabledFeatures,
		PendingFeatures:              site.PendingFeatures,
		LoadBalancedUpstreamID:       site.LoadBalancedUpstreamID,
		Status:                       string(site.Status()),
		InstalledAt:                  pkgdto.FormatTime(site.InstalledAt),
		InstallationFailedAt:         pkgdto.FormatTime(site.InstallationFailedAt),
		UninstallationRequestedAt:    pkgdto.FormatTime(site.UninstallationRequestedAt),
		UninstallationFailedAt:       pkgdto.FormatTime(site.UninstallationFailedAt),
		CreatedAt:                    pkgdto.FormatTimeOrEmpty(site.CreatedAt),
		UpdatedAt:                    pkgdto.FormatTimeOrEmpty(site.UpdatedAt),
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
		CreatedAt:    pkgdto.FormatTimeOrEmpty(deployment.CreatedAt),
		UpdatedAt:    pkgdto.FormatTimeOrEmpty(deployment.UpdatedAt),
	}
}

// ToCertificateResponse converts a Certificate model to a response DTO
func ToCertificateResponse(cert *models.Certificate) CertificateResponse {
	return CertificateResponse{
		ID:         cert.ID,
		SiteID:     cert.SiteID,
		Type:       string(cert.Type),
		Domains:    cert.Domains,
		IsActive:   cert.IsActive,
		UploadedAt: pkgdto.FormatTime(cert.UploadedAt),
		CreatedAt:  pkgdto.FormatTimeOrEmpty(cert.CreatedAt),
	}
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

	resp := QueueResponse{
		ID:                    queue.ID,
		SiteID:                queue.SiteID,
		ServerID:              queue.ServerID,
		Name:                  queue.QueueName,
		Directory:             directory,
		Command:               formatter.QueueCommand(queue),
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
		LastStatusCheck:       pkgdto.FormatTime(queue.LastStatusCheck),
		InstalledAt:           pkgdto.FormatTime(queue.InstalledAt),
		CreatedAt:             pkgdto.FormatTimeOrEmpty(queue.CreatedAt),
	}

	// Parse info JSON if present
	if queue.Info != nil && *queue.Info != "" {
		var info map[string]any
		if err := json.Unmarshal([]byte(*queue.Info), &info); err == nil {
			resp.Info = info
		}
	}

	return resp
}

// ToCommandResponse converts a Command model to a response DTO
func ToCommandResponse(cmd *models.Command) CommandResponse {
	resp := CommandResponse{
		ID:        cmd.ID,
		SiteID:    cmd.SiteID,
		UserID:    cmd.UserID,
		Command:   cmd.Command,
		Status:    string(cmd.Status),
		Output:    cmd.Output,
		ExitCode:  cmd.ExitCode,
		CreatedAt: pkgdto.FormatTimeOrEmpty(cmd.CreatedAt),
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
	typeLabel := redirectTypeLabel(redirect.Mode)

	return RedirectResponse{
		ID:        redirect.ID,
		SiteID:    redirect.SiteID,
		Type:      redirect.Mode,
		TypeLabel: typeLabel,
		From:      redirect.From,
		To:        redirect.To,
		Status:    redirect.Status,
		CreatedAt: pkgdto.FormatTimeOrEmpty(redirect.CreatedAt),
	}
}

func redirectTypeLabel(mode int) string {
	switch mode {
	case 301:
		return "Permanent (301)"
	case 302:
		return "Temporary (302)"
	case 307:
		return "Temporary (307)"
	case 308:
		return "Permanent (308)"
	default:
		return "Unknown"
	}
}

// SiteTypeOption represents a site type option for the create site form
type SiteTypeOption struct {
	Value            string `json:"value"`
	Label            string `json:"label"`
	DefaultWebFolder string `json:"default_web_folder"`
	RequiresGit      bool   `json:"requires_git"`
	SupportsGit      bool   `json:"supports_git"`
}

// CreateSiteOptionsResponse represents the response for site creation options
type CreateSiteOptionsResponse struct {
	SiteTypes []SiteTypeOption `json:"site_types"`
}

// GetCreateSiteOptions builds the create site options response
func GetCreateSiteOptions() CreateSiteOptionsResponse {
	types := sitetypes.AllSiteTypes()
	options := make([]SiteTypeOption, len(types))

	for i, st := range types {
		options[i] = SiteTypeOption{
			Value:            st.String(),
			Label:            st.Label(),
			DefaultWebFolder: st.GetDefaultWebFolder(),
			RequiresGit:      st.RequiresGitAccount(),
			SupportsGit:      st.RequiresGitAccount(),
		}
	}

	return CreateSiteOptionsResponse{
		SiteTypes: options,
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
