package services

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/hibiken/asynq"

	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	gitproviders "github.com/kkz6/launch-go/internal/modules/git/providers"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// DeploymentService handles business logic for deployments
type DeploymentService struct {
	*BaseService
	gitReader       contracts.GitReader
	providerFactory *gitproviders.ProviderFactory
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(deps *ServiceDeps) *DeploymentService {
	return &DeploymentService{
		BaseService: NewBaseService(deps),
	}
}

// SetGitReader sets the git reader for cross-module queries
func (s *DeploymentService) SetGitReader(reader contracts.GitReader) {
	s.gitReader = reader
}

// SetProviderFactory sets the git provider factory
func (s *DeploymentService) SetProviderFactory(factory *gitproviders.ProviderFactory) {
	s.providerFactory = factory
}

// Deploy triggers a new deployment for a site
func (s *DeploymentService) Deploy(ctx context.Context, siteID, serverID, userID string) (*models.Deployment, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	// Fetch the latest commit data from the git provider
	commitData := s.FetchLatestCommitData(ctx, site)

	return s.createDeployment(ctx, site, userID, commitData, nil)
}

// FetchLatestCommitData fetches the latest commit data from the git provider
func (s *DeploymentService) FetchLatestCommitData(ctx context.Context, site *models.Site) map[string]any {
	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return nil
	}

	if s.gitReader == nil || s.providerFactory == nil {
		return nil
	}

	// Get source control
	sourceControl, err := s.gitReader.FindSourceControlByID(ctx, *site.SourceControlID, site.TeamID)
	if err != nil {
		s.LogError(err, "Failed to find source control for commit data", "site_id", site.ID)
		return nil
	}

	// Get repository
	repoID := strconv.FormatUint(*site.SourceControlRepositoriesID, 10)
	repo, err := s.gitReader.FindRepositoryByID(ctx, repoID)
	if err != nil {
		s.LogError(err, "Failed to find repository for commit data", "site_id", site.ID)
		return nil
	}

	// Get provider
	provider, err := s.providerFactory.GetProviderWithInstallation(
		gitproviders.GitProviderType(sourceControl.Provider),
		s.buildSourceControlData(sourceControl),
	)
	if err != nil {
		s.LogError(err, "Failed to get provider for commit data", "site_id", site.ID)
		return nil
	}

	// Get last commit
	branch := site.GetRepositoryBranch()
	commitData, err := provider.GetLastCommit(ctx, sourceControl.ID, repo.FullName, branch)
	if err != nil {
		s.LogError(err, "Failed to get last commit", "site_id", site.ID, "repo", repo.FullName, "branch", branch)
		return nil
	}

	if commitData == nil {
		return nil
	}

	return commitData.ToMap()
}

// buildSourceControlData builds SourceControlData from a SourceControl model
func (s *DeploymentService) buildSourceControlData(sc *gitmodels.SourceControl) *gitproviders.SourceControlData {
	scData := &gitproviders.SourceControlData{
		ID:             sc.ID,
		UserID:         sc.UserID,
		TeamID:         sc.TeamID,
		Provider:       gitproviders.GitProviderType(sc.Provider),
		InstallationID: sc.InstallationID,
	}

	if sc.URL != nil {
		scData.URL = sc.URL
	}
	if sc.Login != nil {
		scData.Login = sc.Login
	}
	if sc.Name != nil {
		scData.Name = sc.Name
	}
	if sc.Type != nil {
		scData.Type = sc.Type
	}

	return scData
}

// Rollback rolls back to a previous deployment
func (s *DeploymentService) Rollback(ctx context.Context, siteID, serverID, targetDeploymentID, userID string) (*models.Deployment, error) {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return nil, err
	}

	if !site.ZeroDowntimeDeployment {
		return nil, ErrRollbackNotSupported
	}

	targetDeployment, err := s.Repos().Deployment().FindByID(ctx, targetDeploymentID)
	if err != nil {
		return nil, err
	}

	if targetDeployment.SiteID != site.ID {
		return nil, ErrDeploymentNotBelongToSite
	}

	if targetDeployment.Status != sitetypes.DeploymentStatusFinished {
		return nil, ErrInvalidRollbackTarget
	}

	// Check for active deployment
	activeDeployment, err := s.Repos().Deployment().FindActiveBySite(ctx, site.ID)
	if err != nil {
		return nil, errors.New("failed to check for active deployment")
	}
	if activeDeployment != nil {
		return nil, ErrPendingDeployment
	}

	// Get latest deployment for rollback metadata
	latestDeployment, err := s.Repos().Deployment().FindLatestBySite(ctx, site.ID)
	if err != nil {
		s.LogWarn("Failed to fetch latest deployment for rollback", "siteID", site.ID, "error", err)
	}

	// Create rollback deployment
	commitData := map[string]any{
		"rollback_to": targetDeployment.ID,
	}
	if latestDeployment != nil {
		commitData["rollback_from"] = latestDeployment.ID
	}

	for k, v := range targetDeployment.CommitData {
		if k != "rollback_from" && k != "rollback_to" {
			commitData[k] = v
		}
	}

	deployment := &models.Deployment{
		UserID:     stringToPtr(userID),
		Status:     sitetypes.DeploymentStatusPending,
		GitHash:    targetDeployment.GitHash,
		CommitData: commitData,
	}
	deployment.SiteID = site.ID
	deployment.TeamID = site.TeamID

	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch rollback job
	task, err := jobs.NewRollbackTask(site.ID, deployment.ID, targetDeploymentID, userID)
	if err != nil {
		return nil, err
	}

	// Add a small delay to ensure the database transaction is fully committed
	if err := s.EnqueueTaskWithOptions(task, asynq.ProcessIn(1*time.Second)); err != nil {
		s.LogError(err, "Failed to enqueue rollback job", "deployment_id", deployment.ID)
	}

	s.LogInfo("Rollback initiated", "site_id", site.ID, "deployment_id", deployment.ID, "target_deployment_id", targetDeploymentID)

	return deployment, nil
}

// createDeployment creates a new deployment for a site.
// envVars are transient environment variables (e.g. database credentials) passed through the job payload
// and never persisted in the database.
func (s *DeploymentService) createDeployment(ctx context.Context, site *models.Site, userID string, commitData map[string]any, envVars map[string]string) (*models.Deployment, error) {
	// Convert empty userID to nil (webhook deployments have no user)
	userIDPtr := stringToPtr(userID)

	// Extract git hash from commit data
	gitHash := extractGitHash(commitData)

	// Check for active deployment
	activeDeployment, err := s.Repos().Deployment().FindActiveBySite(ctx, site.ID)
	if err != nil {
		return nil, errors.New("failed to check for active deployment")
	}
	if activeDeployment != nil {
		if site.QueueDeployments {
			// Queue the deployment
			deployment := &models.Deployment{
				UserID:     userIDPtr,
				Status:     sitetypes.DeploymentStatusQueued,
				GitHash:    gitHash,
				CommitData: commitData,
			}
			deployment.SiteID = site.ID
			deployment.TeamID = site.TeamID

			if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
				return nil, err
			}

			s.LogInfo("Deployment queued", "site_id", site.ID, "deployment_id", deployment.ID)

			return deployment, nil
		}

		return nil, ErrPendingDeployment
	}

	deployment := &models.Deployment{
		UserID:     userIDPtr,
		Status:     sitetypes.DeploymentStatusPending,
		GitHash:    gitHash,
		CommitData: commitData,
	}
	deployment.SiteID = site.ID
	deployment.TeamID = site.TeamID

	if err := s.Repos().Deployment().Create(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error

	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, userID, envVars)
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, userID, envVars)
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	// Add a small delay to ensure the database transaction is fully committed
	// before the job starts processing
	if err := s.EnqueueTaskWithOptions(task, asynq.ProcessIn(1*time.Second)); err != nil {
		s.LogError(err, "Failed to enqueue deployment job", "deployment_id", deployment.ID)
	}

	s.LogInfo("Deployment started", "site_id", site.ID, "deployment_id", deployment.ID)

	s.BroadcastToTeam(site.TeamID, "deployment.started", map[string]any{
		"team_id":       site.TeamID,
		"site_id":       site.ID,
		"deployment_id": deployment.ID,
		"status":        string(deployment.Status),
	})

	return deployment, nil
}

// List returns all deployments for a site
func (s *DeploymentService) List(ctx context.Context, siteID, serverID string) ([]models.Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Deployment().FindBySite(ctx, siteID)
}

// FindByID finds a deployment by ID
func (s *DeploymentService) FindByID(ctx context.Context, id, siteID, serverID string) (*models.Deployment, error) {
	// Verify site exists and belongs to server
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return nil, err
	}

	return s.Repos().Deployment().FindByIDAndSite(ctx, id, siteID)
}

// ProcessNextQueued processes the next queued deployment
func (s *DeploymentService) ProcessNextQueued(ctx context.Context, siteID string) (*models.Deployment, error) {
	site, err := s.Repos().Site().FindByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	// Check for active deployment
	activeDeployment, err := s.Repos().Deployment().FindActiveBySite(ctx, site.ID)
	if err != nil {
		return nil, errors.New("failed to check for active deployment")
	}
	if activeDeployment != nil {
		return nil, nil
	}

	// Get next queued deployment
	queuedDeployments, err := s.Repos().Deployment().FindQueuedBySite(ctx, site.ID)
	if err != nil {
		return nil, err
	}

	if len(queuedDeployments) == 0 {
		return nil, nil
	}

	deployment := &queuedDeployments[0]
	deployment.Status = sitetypes.DeploymentStatusPending

	if err := s.Repos().Deployment().Update(ctx, deployment); err != nil {
		return nil, err
	}

	// Dispatch deployment job
	var task *asynq.Task
	var dispatchErr error

	if site.ZeroDowntimeDeployment {
		task, dispatchErr = jobs.NewDeployZeroDowntimeTask(site.ID, deployment.ID, "", nil)
	} else {
		task, dispatchErr = jobs.NewDeployTask(site.ID, deployment.ID, "", nil)
	}

	if dispatchErr != nil {
		return nil, dispatchErr
	}

	if err := s.EnqueueTask(task); err != nil {
		s.LogError(err, "Failed to enqueue deployment job", "deployment_id", deployment.ID)
	}

	return deployment, nil
}

// GetQueuedCount returns the count of queued deployments
func (s *DeploymentService) GetQueuedCount(ctx context.Context, siteID string) (int64, error) {
	return s.Repos().Deployment().CountQueuedBySite(ctx, siteID)
}

// CancelQueued cancels all queued deployments
func (s *DeploymentService) CancelQueued(ctx context.Context, siteID, serverID string) (int64, error) {
	// Verify site exists and belongs to server
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return 0, err
	}

	return s.Repos().Deployment().CancelQueued(ctx, siteID)
}

// EnableAutoDeployment enables auto-deployment for a site
func (s *DeploymentService) EnableAutoDeployment(ctx context.Context, siteID, serverID string) error {
	site, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID)
	if err != nil {
		return err
	}

	if site.SourceControlID == nil || site.SourceControlRepositoriesID == nil {
		return ErrSourceControlNotConnected
	}

	site.AutoDeployment = true

	return s.Repos().Site().Update(ctx, site)
}

// DisableAutoDeployment disables auto-deployment for a site
func (s *DeploymentService) DisableAutoDeployment(ctx context.Context, siteID, serverID string) error {
	if _, err := s.Repos().Site().FindByIDAndServer(ctx, siteID, serverID); err != nil {
		return err
	}

	return s.Repos().Site().UpdateFields(ctx, siteID, map[string]any{
		"auto_deployment": false,
	})
}

// BroadcastProgress broadcasts deployment progress
func (s *DeploymentService) BroadcastProgress(siteID, deploymentID, status, message string) {
	s.BroadcastToDeployment(deploymentID, "deployment.progress", map[string]any{
		"site_id":       siteID,
		"deployment_id": deploymentID,
		"status":        status,
		"message":       message,
	})
}

// DeployFromWebhook triggers a deployment from a git provider webhook
// This is called without authentication - the deploy token serves as auth
func (s *DeploymentService) DeployFromWebhook(ctx context.Context, siteID, token string, payload map[string]any) error {
	// Find site by ID
	site, err := s.Repos().Site().FindByID(ctx, siteID)
	if err != nil {
		return errors.New("site not found")
	}

	// Validate deploy token
	if site.DeployToken == nil || !security.SecureCompare(*site.DeployToken, token) {
		return ErrInvalidDeployToken
	}

	// Extract commit data from webhook payload
	commitData := s.parseWebhookPayload(payload, site)

	// Check branch match if we have commit data
	if commitData != nil {
		if branch, ok := commitData["branch"].(string); ok {
			siteBranch := "main"
			if site.RepositoryBranch != nil && *site.RepositoryBranch != "" {
				siteBranch = *site.RepositoryBranch
			}
			if branch != siteBranch {
				return ErrBranchMismatch
			}
		}
	}

	// Create deployment (no user ID for webhook-triggered deployments)
	_, err = s.createDeployment(ctx, site, "", commitData, nil)
	if err != nil {
		return err
	}

	return nil
}

// parseWebhookPayload extracts commit data from git provider webhook payloads
func (s *DeploymentService) parseWebhookPayload(payload map[string]any, site *models.Site) map[string]any {
	if payload == nil {
		return nil
	}

	// Try GitHub format
	if headCommit, ok := payload["head_commit"].(map[string]any); ok {
		return s.parseGitHubPayload(payload, headCommit)
	}

	// Try GitLab format
	if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 {
		if commit, ok := commits[0].(map[string]any); ok {
			return s.parseGitLabPayload(payload, commit)
		}
	}

	// Try Bitbucket format
	if push, ok := payload["push"].(map[string]any); ok {
		return s.parseBitbucketPayload(push)
	}

	return nil
}

// parseGitHubPayload parses GitHub webhook payload
func (s *DeploymentService) parseGitHubPayload(payload, commit map[string]any) map[string]any {
	result := make(map[string]any)

	if id, ok := commit["id"].(string); ok {
		result["commit_id"] = id
		if len(id) >= 7 {
			result["sha"] = id[:7]
		}
	}

	if author, ok := commit["author"].(map[string]any); ok {
		if name, ok := author["name"].(string); ok {
			result["name"] = name
		}
		if email, ok := author["email"].(string); ok {
			result["email"] = email
		}
	}

	if message, ok := commit["message"].(string); ok {
		result["message"] = message
	}

	if url, ok := commit["url"].(string); ok {
		result["url"] = url
	}

	// Extract branch from ref (refs/heads/main -> main)
	if ref, ok := payload["ref"].(string); ok {
		const prefix = "refs/heads/"
		if len(ref) > len(prefix) {
			result["branch"] = ref[len(prefix):]
		}
	}

	return result
}

// parseGitLabPayload parses GitLab webhook payload
func (s *DeploymentService) parseGitLabPayload(payload, commit map[string]any) map[string]any {
	result := make(map[string]any)

	if id, ok := commit["id"].(string); ok {
		result["commit_id"] = id
		if len(id) >= 7 {
			result["sha"] = id[:7]
		}
	}

	if author, ok := commit["author"].(map[string]any); ok {
		if name, ok := author["name"].(string); ok {
			result["name"] = name
		}
		if email, ok := author["email"].(string); ok {
			result["email"] = email
		}
	}

	if message, ok := commit["message"].(string); ok {
		result["message"] = message
	}

	if url, ok := commit["url"].(string); ok {
		result["url"] = url
	}

	// Extract branch from ref
	if ref, ok := payload["ref"].(string); ok {
		const prefix = "refs/heads/"
		if len(ref) > len(prefix) {
			result["branch"] = ref[len(prefix):]
		} else {
			result["branch"] = ref
		}
	}

	return result
}

// parseBitbucketPayload parses Bitbucket webhook payload
func (s *DeploymentService) parseBitbucketPayload(push map[string]any) map[string]any {
	changes, ok := push["changes"].([]any)
	if !ok || len(changes) == 0 {
		return nil
	}

	change, ok := changes[0].(map[string]any)
	if !ok {
		return nil
	}

	commits, ok := change["commits"].([]any)
	if !ok || len(commits) == 0 {
		return nil
	}

	commit, ok := commits[0].(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[string]any)

	if hash, ok := commit["hash"].(string); ok {
		result["commit_id"] = hash
		if len(hash) >= 7 {
			result["sha"] = hash[:7]
		}
	}

	if author, ok := commit["author"].(map[string]any); ok {
		if raw, ok := author["raw"].(string); ok {
			result["name"] = raw
		}
	}

	if message, ok := commit["message"].(string); ok {
		result["message"] = message
	}

	if links, ok := commit["links"].(map[string]any); ok {
		if html, ok := links["html"].(map[string]any); ok {
			if href, ok := html["href"].(string); ok {
				result["url"] = href
			}
		}
	}

	// Extract branch
	if newTarget, ok := change["new"].(map[string]any); ok {
		if name, ok := newTarget["name"].(string); ok {
			result["branch"] = name
		}
	}

	return result
}

// extractGitHash extracts the git hash from commit data
func extractGitHash(commitData map[string]any) *string {
	if commitData == nil {
		return nil
	}

	// Get sha from commit data
	if sha, ok := commitData["sha"].(string); ok && sha != "" {
		return &sha
	}

	return nil
}
