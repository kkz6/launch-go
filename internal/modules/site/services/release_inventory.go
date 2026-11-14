package services

import (
	"context"
	"errors"
	"regexp"
	"time"

	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

var (
	inventoryReleaseName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	inventoryLegacyRelease = regexp.MustCompile(`(?m)^DEPLOYMENT_KEEP="([0-9]{14})"$`)
)

// ReleaseInventory reads the actual current link and retained directories under
// the site's deployment lock. History is used only to identify those releases.
func (s *DeploymentService) ReleaseInventory(ctx context.Context, siteID, serverID, teamID string) (*dto.ReleaseInventoryResponse, error) {
	site, err := s.scopedSite(ctx, siteID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	var server servermodels.Server
	if err := s.DB().WithContext(ctx).Where("id = ? AND team_id = ?", serverID, teamID).First(&server).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	response := &dto.ReleaseInventoryResponse{
		Status: "unavailable", CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), Releases: []dto.RetainedRelease{},
	}
	if !site.ZeroDowntimeDeployment {
		response.Status = "unsupported"
		return response, nil
	}
	if s.releaseProbe == nil {
		return response, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	result, err := s.releaseProbe(probeCtx, &server, site)
	response.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err != nil || result == nil || probeCtx.Err() != nil {
		return response, nil
	}
	if result.ExitCode == tasks.ReleaseInventoryBusyExitCode {
		response.Status = "busy"
		return response, nil
	}
	if result.ExitCode != 0 {
		return response, nil
	}
	observation, err := tasks.ParseReleaseInventory(result.Stdout)
	if err != nil {
		return response, nil
	}
	var deployments []models.Deployment
	if err := s.DB().WithContext(ctx).Where("site_id = ? AND team_id = ?", siteID, teamID).
		Order("created_at DESC, id DESC").Find(&deployments).Error; err != nil {
		return nil, err
	}
	identities, err := s.inventoryDeploymentIdentities(ctx, deployments, serverID, siteID)
	if err != nil {
		return nil, err
	}
	response.Status = "available"
	if observation.ActiveRelease != "" {
		response.ActiveRelease = &observation.ActiveRelease
	}
	for _, name := range observation.Releases {
		release := dto.RetainedRelease{ReleaseName: name, DeploymentIDs: []string{}, Active: name == observation.ActiveRelease}
		for _, deployment := range deployments {
			if identities[deployment.ID] != name {
				continue
			}
			release.DeploymentIDs = append(release.DeploymentIDs, deployment.ID)
			if release.Status == "" {
				release.Status = string(deployment.Status)
			}
			// Prefer the newest completed operation for each physical release.
			if release.DeploymentID == "" && deployment.Status == sitetypes.DeploymentStatusFinished {
				release.DeploymentID = deployment.ID
				release.Status = string(deployment.Status)
			}
		}
		switch {
		case release.Active:
			release.UnavailableReason = "active"
		case len(release.DeploymentIDs) == 0:
			release.UnavailableReason = "unknown_deployment"
		case release.DeploymentID == "":
			release.UnavailableReason = "unfinished"
		default:
			release.RollbackAvailable = true
		}
		response.Releases = append(response.Releases, release)
	}
	return response, nil
}

// inventoryDeploymentIdentities recovers legacy identities from saved task
// scripts in batches. Reading inventory never persists or guesses an identity.
func (s *DeploymentService) inventoryDeploymentIdentities(ctx context.Context, deployments []models.Deployment, serverID, siteID string) (map[string]string, error) {
	identities := make(map[string]string)
	taskIDs := []string{}
	for _, deployment := range deployments {
		if deployment.ReleaseName != nil {
			if inventoryReleaseName.MatchString(*deployment.ReleaseName) {
				identities[deployment.ID] = *deployment.ReleaseName
			}
		} else if deployment.TaskID != nil {
			taskIDs = append(taskIDs, *deployment.TaskID)
		}
	}
	legacy := make(map[string]string)
	for start := 0; start < len(taskIDs); start += 500 {
		end := min(start+500, len(taskIDs))
		var savedTasks []servermodels.Task
		if err := s.DB().WithContext(ctx).Select("id", "script").Where("id IN ? AND server_id = ? AND (site_id = ? OR site_id IS NULL)", taskIDs[start:end], serverID, siteID).Find(&savedTasks).Error; err != nil {
			return nil, err
		}
		for _, task := range savedTasks {
			matches := inventoryLegacyRelease.FindAllStringSubmatch(string(task.Script), -1)
			if len(matches) == 1 {
				legacy[task.ID] = matches[0][1]
			}
		}
	}
	for _, deployment := range deployments {
		if deployment.ReleaseName == nil && deployment.TaskID != nil && legacy[*deployment.TaskID] != "" {
			identities[deployment.ID] = legacy[*deployment.TaskID]
		}
	}
	return identities, nil
}

type releaseInventoryProbe func(context.Context, *servermodels.Server, *models.Site) (*taskrunner.SSHCommandResult, error)

func probeSiteReleases(ctx context.Context, server *servermodels.Server, site *models.Site) (*taskrunner.SSHCommandResult, error) {
	client, err := server.ConnectionAsUser(site.User).NewSSHClient(3 * time.Second)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if err := client.ConnectContext(ctx); err != nil {
		return nil, err
	}
	// Send the quoted script directly: a GET must not upload scripts or leave
	// logs on the server. The remote timeout also bounds abandoned connections.
	script := tasks.NewReleaseInventoryTask(site.Path).Script()
	return client.Run(ctx, "timeout 10s bash -c "+taskrunner.ShellQuote(script))
}

// ValidateRollbackRelease performs a fresh preflight; the rollback task still
// validates the target again under its exclusive lock before changing current.
func (s *DeploymentService) ValidateRollbackRelease(ctx context.Context, siteID, serverID, teamID, targetID string) (*dto.ReleaseInventoryResponse, error) {
	inventory, err := s.ReleaseInventory(ctx, siteID, serverID, teamID)
	if err != nil {
		return nil, err
	}
	if inventory.Status != "available" {
		return nil, fiberutil.Conflict("Release state could not be verified. Wait for any active deployment to finish and try again.")
	}
	for _, release := range inventory.Releases {
		for _, id := range release.DeploymentIDs {
			if id != targetID {
				continue
			}
			if release.Active {
				return nil, fiberutil.Conflict("This release is already live")
			}
			if release.RollbackAvailable && release.DeploymentID == targetID {
				return inventory, nil
			}
		}
	}
	return nil, fiberutil.Conflict("This release is unavailable for rollback. It may have been removed or has no verified deployment identity.")
}
