package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ListServers returns all servers for a team. Signature matches IndexFunc.
func (s *Service) ListServers(ctx context.Context, teamID string) ([]dto.ServerResponse, error) {
	servers, err := s.repos.Server().FindAllByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ServerResponse, len(servers))
	for i := range servers {
		out[i] = dto.ToServerResponse(&servers[i])
	}
	return out, nil
}

// ListArchivedServers returns all archived servers for a team. Signature
// matches IndexFunc.
func (s *Service) ListArchivedServers(ctx context.Context, teamID string) ([]dto.ServerResponse, error) {
	servers, err := s.repos.Server().FindArchivedByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ServerResponse, len(servers))
	for i := range servers {
		out[i] = dto.ToServerResponse(&servers[i])
	}
	return out, nil
}

// ListServersPaginated returns servers with pagination.
func (s *Service) ListServersPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error) {
	return s.repos.Server().FindAllByTeamPaginated(ctx, teamID, page, perPage)
}

// GetServer returns a server (with relations) by ID and returns the
// response DTO. Signature matches ShowFunc.
func (s *Service) GetServer(ctx context.Context, id, teamID string) (dto.ServerResponse, error) {
	server, err := s.repos.Server().FindWithRelations(ctx, id, teamID)
	if err != nil {
		return dto.ServerResponse{}, err
	}
	return dto.ToServerResponse(server), nil
}

// GetServerRaw returns a server model by ID. Used by handlers that need
// the raw model (e.g. site-count verification).
func (s *Service) GetServerRaw(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
}

// GetServerWithRelations returns a server model with relations. Used by
// handlers that need access to nested fields (e.g. log_handler).
func (s *Service) GetServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindWithRelations(ctx, id, teamID)
}

// CreateServer creates a new server and returns the response DTO.
// Signature matches CreateFunc.
func (s *Service) CreateServer(ctx context.Context, teamID, userID string, req *dto.CreateServerRequest) (dto.ServerResponse, error) {
	server, err := s.buildAndDispatchServer(ctx, teamID, userID, req)
	if err != nil {
		return dto.ServerResponse{}, err
	}
	return dto.ToServerResponse(server), nil
}

func (s *Service) buildAndDispatchServer(ctx context.Context, teamID, userID string, req *dto.CreateServerRequest) (*models.Server, error) {
	provider, err := types.ParseServerProvider(req.Provider)
	if err != nil {
		return nil, ErrInvalidProvider
	}

	serverType, err := types.ParseServerType(req.Type)
	if err != nil {
		return nil, ErrInvalidServerType
	}

	os := types.OSUbuntu24
	if req.OperatingSystem != "" {
		os, err = types.ParseOperatingSystem(req.OperatingSystem)
		if err != nil {
			os = types.OSUbuntu24
		}
	}

	privateKey, publicKey, err := GenerateSSHKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	serverTypeStr := serverType.String()
	osStr := os.String()
	defaultSSHPort := 22
	defaults := config.ServerDefaults()
	defaultUsername := defaults.Username

	// Generate passwords for system user and database (like Laravel's Str::password)
	password := security.GeneratePassword(32)
	databasePassword := security.GeneratePassword(32)

	// Custom (BYO) servers sit in awaiting_connection until the user runs
	// the provision script and clicks "Try Connection". Cloud servers go
	// through the auto-polling path (status=new, advanced by the
	// WaitForServerToConnect job once the VM boots).
	initialStatus := types.ServerStatusNew
	if provider == types.ProviderCustom {
		initialStatus = types.ServerStatusAwaitingConnection
	}

	server := &models.Server{
		Name:             req.Name,
		Description:      req.Description,
		Provider:         provider,
		Type:             &serverTypeStr,
		OperatingSystem:  &osStr,
		Status:           initialStatus,
		SSHPort:          &defaultSSHPort,
		Username:         &defaultUsername,
		Password:         dbtype.EncryptedString(password),
		DatabasePassword: dbtype.EncryptedString(databasePassword),
		PrivateKey:       dbtype.EncryptedString(privateKey),
		PublicKey:        dbtype.EncryptedString(publicKey),
	}
	server.TeamID = teamID
	server.UserID = userID

	if req.SSHPort > 0 {
		server.SSHPort = &req.SSHPort
	}

	if req.SSHUser != "" {
		server.Username = &req.SSHUser
	}

	if provider == types.ProviderCustom {
		server.PublicIPv4 = &req.IP
	}

	workingDir := ".launch"
	server.WorkingDirectory = &workingDir

	// Set provider data for cloud servers
	if provider != types.ProviderCustom {
		server.ServerProviderID = &req.CredentialID
		server.ProviderData = map[string]interface{}{
			"region": req.Region,
			"plan":   req.Size,
		}
	}

	if err := s.repos.Server().Create(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Create services based on server type (like Laravel's createServices)
	if err := s.createServicesForServer(ctx, server, req); err != nil {
		s.LogError(err, "Failed to create services for server", "server_id", server.ID)
	}

	// Create default firewall rules (SSH, HTTP, HTTPS)
	if err := s.CreateDefaultFirewallRules(ctx, server.ID); err != nil {
		s.LogError(err, "Failed to create default firewall rules", "server_id", server.ID)
	}

	// Update progress to 15% (matches Laravel's flow)
	if err := s.repos.Server().UpdateProgress(ctx, server.ID, 15, "installing updates"); err != nil {
		s.LogError(err, "Failed to update server progress", "server_id", server.ID)
	}

	// Attach SSH keys to server
	if len(req.SSHKeyIDs) > 0 {
		for _, keyID := range req.SSHKeyIDs {
			if err := s.repos.SSHKey().AttachToServer(ctx, server.ID, keyID); err != nil {
				s.LogError(err, "Failed to attach SSH key to server", "server_id", server.ID, "key_id", keyID)
			}
		}
	}

	activity.RecordCreated(ctx, userID, server, "Server was created")

	// Custom servers don't dispatch any job here — the user must paste the
	// provision script into their box and then trigger TryConnection from
	// the UI. Cloud servers need the VM created on the provider first;
	// WaitForServerToConnect then auto-polls and dispatches provisioning.
	if provider != types.ProviderCustom {
		if err := s.dispatchCreateOnProviderJob(server, req.CredentialID, req.SSHKeyIDs); err != nil {
			s.LogError(err, "Failed to dispatch create on provider job", "server_id", server.ID)
		}
	}

	return server, nil
}

// UpdateServer updates a server and returns the response DTO. Signature
// matches UpdateFunc.
func (s *Service) UpdateServer(ctx context.Context, id, teamID, userID string, req *dto.UpdateServerRequest) (dto.ServerResponse, error) {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return dto.ServerResponse{}, err
	}

	if req.Name != nil {
		server.Name = *req.Name
	}

	if req.Description != nil {
		server.Description = req.Description
	}

	if req.MonitoringEnabled != nil {
		server.MonitoringEnabled = *req.MonitoringEnabled
	}

	if req.AutoUpdate != nil {
		server.AutoUpdate = *req.AutoUpdate
	}

	if req.SSHPort != nil {
		server.SSHPort = req.SSHPort
	}

	if err := s.repos.Server().Update(ctx, server); err != nil {
		return dto.ServerResponse{}, err
	}

	activity.RecordEvent(ctx, "updated", userID, server, "Server was updated")
	s.broadcastServerUpdate(server)
	return dto.ToServerResponse(server), nil
}

// DeleteServer deletes a server. Signature matches DeleteFunc.
//
// Broadcasting contract:
//   - Immediately after marking status=deleting we broadcast `server.updated`
//     so the UI flips the badge to "Deleting" without waiting for the worker.
//     Without this, the user clicks Delete, nothing visible happens for
//     seconds (cloud delete API can take 10-30s), and they assume sockets
//     are broken.
//   - For custom providers we delete the row inline and broadcast
//     `server.deleted` so the list re-fetches and the card disappears.
//   - For cloud providers, DeleteServerJob.Handle owns the final
//     `server.deleted` broadcast after the upstream API call succeeds.
//     DeleteServerJob.Failed broadcasts `server.deletion_failed` so the UI
//     can surface the error rather than spinning forever.
func (s *Service) DeleteServer(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// Docker servers refuse deletion while live projects still belong to
	// them. Mirrors how ProjectService.DeleteProject refuses to delete a
	// project that still has workloads — the user must tear things down
	// from the bottom up. Querying the docker_projects table directly
	// (rather than importing the docker repo) keeps the server module
	// independent of the docker module's import graph.
	//
	// The PHP server types (php / database / loadbalancer) don't have
	// docker_projects rows by construction, so the COUNT comes back zero
	// and the check is free for them — no `if server.Type == "docker"`
	// guard needed.
	if err := s.guardDockerProjectsExist(ctx, server.ID); err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", "", server, "Server deletion requested")

	if err := s.repos.Server().UpdateStatus(ctx, id, types.ServerStatusDeleting); err != nil {
		return err
	}

	// Reload so the broadcast payload reflects the new status.
	if updated, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err == nil {
		s.broadcastServerUpdate(updated)
	}

	if server.Provider == types.ProviderCustom {
		if err := s.repos.Server().Delete(ctx, id); err != nil {
			return err
		}
		s.BroadcastToTeam(server.TeamID, "server.deleted", map[string]any{
			"server_id": server.ID,
		})
		return nil
	}

	if err := s.dispatchDeleteJob(server); err != nil {
		s.LogError(err, "Failed to dispatch delete job", "server_id", server.ID)
	}
	return nil
}

// ArchiveServer archives a server. Signature matches ActionFunc.
func (s *Service) ArchiveServer(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err != nil {
		return err
	}
	return s.repos.Server().Archive(ctx, id)
}

// UnarchiveServer unarchives a server. Signature matches ActionFunc.
func (s *Service) UnarchiveServer(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	if _, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID); err != nil {
		return err
	}
	return s.repos.Server().Unarchive(ctx, id)
}

// RebootServer initiates a server reboot. Signature matches ActionFunc.
func (s *Service) RebootServer(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}
	task, err := jobs.NewRebootServerTask(server.ID, nil)
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

// ConnectServer tests the connection to a server. Signature matches ActionFunc.
func (s *Service) ConnectServer(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
		return errors.New("server has no IP address")
	}

	if server.PrivateKey.IsEmpty() {
		return errors.New("server has no private key")
	}

	client, err := server.ConnectionAsRoot().Dial()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	result, err := client.Run(ctx, "whoami")
	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d", result.ExitCode)
	}

	now := time.Now()
	return s.repos.Server().UpdateFields(ctx, id, map[string]interface{}{
		"connected":               true,
		"last_connectivity_check": now,
	})
}

// TryConnection performs a one-shot SSH check on a custom server sitting in
// awaiting_connection. Triggered by the user clicking "Try Connection" in
// the UI after they've pasted the provision script. On success the server
// advances to provisioning (ProvisionServer job dispatched). On failure no
// state changes — the caller surfaces the error and the user clicks again.
//
// Signature matches ActionFunc so it can be wired up via fiberutil.Action.
func (s *Service) TryConnection(ctx context.Context, id, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	// Validation/state errors below return proper 4xx status codes via the
	// fiberutil helpers instead of plain errors.New. Without the helpers
	// Fiber's default handler maps every Go error to 500, which surfaces
	// as a generic "Internal Server Error" on the client — useless to the
	// customer trying to figure out why their box wasn't reachable.
	if server.Provider != types.ProviderCustom {
		return fiberutil.Validation("Try Connection only applies to custom servers.")
	}

	if server.Status != types.ServerStatusAwaitingConnection {
		// Most common cause: a previous Try Connection already succeeded
		// and the server has moved on to starting/provisioning. 409 is
		// the right shape for "the resource state doesn't allow this
		// action right now."
		return fiberutil.Conflict(fmt.Sprintf("Server is not awaiting connection (current status: %s).", server.Status))
	}

	if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
		return fiberutil.Validation("Server has no IP address recorded yet.")
	}

	if server.PrivateKey.IsEmpty() {
		return fiberutil.Validation("Server has no private key recorded yet.")
	}

	// Single SSH attempt with a tight timeout. The HTTP request is already
	// bounded — we just want to fail fast so the user gets a real answer
	// rather than waiting on a hung TCP.
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := server.ConnectionAsRoot().Dial()
	if err != nil {
		// Customer-fixable (provision script didn't run, firewall blocking
		// SSH, wrong IP). Surface the underlying network error so the
		// toast tells them what to fix instead of "Internal Server Error".
		return fiberutil.Validation(fmt.Sprintf("Could not reach server: %s. Make sure the provision command ran successfully and SSH is reachable.", err.Error()))
	}
	defer client.Close()

	result, err := client.Run(dialCtx, "whoami")
	if err != nil {
		return fiberutil.Validation(fmt.Sprintf("SSH command failed: %s.", err.Error()))
	}

	if result.ExitCode != 0 {
		return fiberutil.Validation(fmt.Sprintf("SSH whoami returned exit code %d.", result.ExitCode))
	}

	now := time.Now()
	if err := s.repos.Server().UpdateFields(ctx, id, map[string]any{
		"connected":               true,
		"last_connectivity_check": now,
		"status":                  types.ServerStatusStarting,
	}); err != nil {
		return fmt.Errorf("failed to persist connection state: %w", err)
	}

	// Reload so the broadcast carries the new status.
	if updated, ferr := s.repos.Server().FindByID(ctx, id); ferr == nil {
		s.BroadcastToTeam(server.TeamID, "server.connected", map[string]any{
			"server_id": updated.ID,
			"status":    string(updated.Status),
		})
		s.broadcastServerUpdate(updated)
	}

	// Hand off to provisioning. Collect the SSH key ids the same way
	// RetryProvision does so deploy keys land on the box.
	sshKeys, _ := s.repos.SSHKey().FindByServer(ctx, id)
	var sshKeyIDs []string
	for _, k := range sshKeys {
		sshKeyIDs = append(sshKeyIDs, k.ID)
	}

	task, err := jobs.NewProvisionServerTask(id, teamID, nil, sshKeyIDs)
	if err != nil {
		return fmt.Errorf("failed to create provision task: %w", err)
	}
	if err := s.EnqueueTask(task); err != nil {
		return fmt.Errorf("failed to enqueue provision task: %w", err)
	}

	activity.RecordEvent(ctx, "connection_established", "", server, "User confirmed SSH connection; provisioning queued")
	return nil
}

// HasLaunchAgent checks if a server has the Launch Agent installed
func (s *Service) HasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	return s.repos.Server().HasLaunchAgent(ctx, serverID)
}

// GetShowPageData returns all data needed for the server show page
func (s *Service) GetShowPageData(ctx context.Context, serverID, teamID string) (*dto.ServerShowPageData, error) {
	server, err := s.repos.Server().FindWithRelations(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}

	var (
		latestTask     *models.Task
		latestMetric   *models.Metric
		hasLaunchAgent bool
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		latestTask, err = s.repos.Task().FindLatestByServer(gctx, serverID)
		if err != nil {
			s.LogWarn("Failed to fetch latest task for server", "serverID", serverID, "error", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		latestMetric, err = s.repos.Metric().FindLatestByServer(gctx, serverID)
		if err != nil {
			s.LogWarn("Failed to fetch latest metric for server", "serverID", serverID, "error", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		hasLaunchAgent, err = s.repos.Server().HasLaunchAgent(gctx, serverID)
		if err != nil {
			s.LogWarn("Failed to check launch agent status", "serverID", serverID, "error", err)
		}
		return nil
	})

	_ = g.Wait()

	services := make([]dto.ServiceResponse, len(server.Services))
	for i, svc := range server.Services {
		services[i] = dto.ToServiceResponse(&svc)
	}

	firewallRules := make([]dto.FirewallRuleResponse, len(server.FirewallRules))
	for i, rule := range server.FirewallRules {
		firewallRules[i] = dto.ToFirewallRuleResponse(&rule)
	}

	crons := make([]dto.CronResponse, len(server.Crons))
	for i, cron := range server.Crons {
		crons[i] = dto.ToCronResponse(&cron)
	}

	daemons := make([]dto.DaemonResponse, len(server.Daemons))
	for i, daemon := range server.Daemons {
		daemons[i] = dto.ToDaemonResponse(&daemon)
	}

	sshKeys := make([]dto.SSHKeyResponse, len(server.SSHKeys))
	for i, key := range server.SSHKeys {
		sshKeys[i] = dto.ToSSHKeyResponse(&key)
	}

	data := &dto.ServerShowPageData{
		Server:         dto.ToServerResponse(server),
		Services:       services,
		FirewallRules:  firewallRules,
		Crons:          crons,
		Daemons:        daemons,
		SSHKeys:        sshKeys,
		RuleActions:    dto.GetAllRuleActions(),
		HasLaunchAgent: hasLaunchAgent,
	}

	if latestTask != nil {
		taskResp := dto.ToTaskResponse(latestTask)
		data.LatestTask = &taskResp
	}

	if latestMetric != nil {
		metricResp := dto.ToMetricResponse(latestMetric)
		data.LatestMetric = &metricResp
	}

	return data, nil
}

func (s *Service) broadcastServerUpdate(server *models.Server) {
	s.BroadcastToTeam(server.TeamID, "server.updated", dto.ToServerResponse(server))
}

func (s *Service) dispatchCreateOnProviderJob(server *models.Server, serverProviderID string, sshKeyIDs []string) error {
	return s.MustDispatchWithOptions(func() (*asynq.Task, error) {
		return jobs.NewCreateOnProviderTask(server.ID, server.TeamID, serverProviderID, nil, sshKeyIDs)
	})
}

func (s *Service) dispatchDeleteJob(server *models.Server) error {
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewDeleteServerTask(server.ID, server.TeamID, nil)
	})
}

// createServicesForServer creates the initial services for a server based on its type and configuration.
// This mirrors Laravel's ServerType::createServices() method.
func (s *Service) createServicesForServer(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
	serverType, _ := types.ParseServerType(req.Type)

	switch serverType {
	case types.ServerTypeDatabase:
		return s.createDatabaseServerServices(ctx, server, req)
	case types.ServerTypeLoadBalancer:
		return s.createLoadBalancerServerServices(ctx, server, req)
	case types.ServerTypeDocker:
		return s.createDockerServerServices(ctx, server, req)
	default:
		// Default to PHP server type
		return s.createPhpServerServices(ctx, server, req)
	}
}

// createPhpServerServices creates services for a PHP server type (matches Laravel PhpServerType::createServices)
func (s *Service) createPhpServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
	// Base services for PHP server: Supervisor, Caddy (NO Redis by default)
	// Composer is only added if PHP is being installed
	baseSoftware := []types.Software{
		types.SoftwareSupervisor,
		types.SoftwareCaddy2,
	}

	for _, sw := range baseSoftware {
		if err := s.createService(ctx, server.ID, sw, false); err != nil {
			return fmt.Errorf("failed to create service %s: %w", sw, err)
		}
	}

	// Add database if not "none"
	if req.DatabaseType != "none" {
		var dbSoftware types.Software
		if req.DatabaseType != "" {
			var err error
			dbSoftware, err = types.ParseSoftware(req.DatabaseType)
			if err != nil {
				dbSoftware = types.SoftwareMySQL80
			}
		} else {
			// Default to MySQL 8.0 if not specified
			dbSoftware = types.SoftwareMySQL80
		}
		if err := s.createService(ctx, server.ID, dbSoftware, false); err != nil {
			return fmt.Errorf("failed to create database service: %w", err)
		}
	}

	// Add PHP and Composer if PHP is not "none"
	// Composer requires PHP, so they're installed together
	if req.PHPVersion != "none" {
		var phpSoftware types.Software
		if req.PHPVersion != "" {
			var err error
			phpSoftware, err = types.ParseSoftware(req.PHPVersion)
			if err != nil {
				phpSoftware = types.SoftwarePhp83
			}
		} else {
			// Default to PHP 8.3 if not specified
			phpSoftware = types.SoftwarePhp83
		}
		if err := s.createService(ctx, server.ID, phpSoftware, true); err != nil {
			return fmt.Errorf("failed to create PHP service: %w", err)
		}

		// Add Composer (requires PHP)
		if err := s.createService(ctx, server.ID, types.SoftwareComposer2, false); err != nil {
			return fmt.Errorf("failed to create Composer service: %w", err)
		}
	}

	// Add Launch Agent if install_agent is not explicitly false
	if req.InstallAgent == nil || *req.InstallAgent {
		if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent, false); err != nil {
			return fmt.Errorf("failed to create Launch Agent service: %w", err)
		}
	}

	return nil
}

// createDatabaseServerServices creates services for a Database server type (matches Laravel DatabaseServerType::createServices)
func (s *Service) createDatabaseServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
	// Add database if not "none"
	if req.DatabaseType != "none" {
		var dbSoftware types.Software
		if req.DatabaseType != "" {
			var err error
			dbSoftware, err = types.ParseSoftware(req.DatabaseType)
			if err != nil {
				dbSoftware = types.SoftwareMySQL80
			}
		} else {
			dbSoftware = types.SoftwareMySQL80
		}
		if err := s.createService(ctx, server.ID, dbSoftware, false); err != nil {
			return fmt.Errorf("failed to create database service: %w", err)
		}
	}

	// Add Launch Agent if install_agent is not explicitly false
	if req.InstallAgent == nil || *req.InstallAgent {
		if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent, false); err != nil {
			return fmt.Errorf("failed to create Launch Agent service: %w", err)
		}
	}

	return nil
}

// createDockerServerServices creates services for a Docker server type.
// Records Docker + Traefik as installed services so the UI surfaces them
// under Services. They are marked Running by the ProvisionDockerServer callback.
func (s *Service) createDockerServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
	if err := s.createService(ctx, server.ID, types.SoftwareDocker, true); err != nil {
		return fmt.Errorf("failed to create Docker service: %w", err)
	}
	if err := s.createService(ctx, server.ID, types.SoftwareTraefik, true); err != nil {
		return fmt.Errorf("failed to create Traefik service: %w", err)
	}

	if req.InstallAgent == nil || *req.InstallAgent {
		if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent, false); err != nil {
			return fmt.Errorf("failed to create Launch Agent service: %w", err)
		}
	}

	return nil
}

// createLoadBalancerServerServices creates services for a Load Balancer server type
func (s *Service) createLoadBalancerServerServices(ctx context.Context, server *models.Server, req *dto.CreateServerRequest) error {
	// Load balancer only needs Caddy configured for reverse proxy
	if err := s.createService(ctx, server.ID, types.SoftwareCaddy2LB, false); err != nil {
		return fmt.Errorf("failed to create Caddy LB service: %w", err)
	}

	// Add Launch Agent if not explicitly disabled
	if req.InstallAgent == nil || *req.InstallAgent {
		if err := s.createService(ctx, server.ID, types.SoftwareLaunchAgent, false); err != nil {
			return fmt.Errorf("failed to create Launch Agent service: %w", err)
		}
	}

	return nil
}

// createService creates a single service record for a server
func (s *Service) createService(ctx context.Context, serverID string, software types.Software, isDefault bool) error {
	service := &models.InstalledService{
		Type:      software.GetServiceType(),
		Name:      software.Label(),
		Version:   software.GetVersion(),
		Status:    types.ServiceStatusPending,
		Software:  software.String(),
		IsDefault: isDefault,
	}
	service.ServerID = serverID

	return s.repos.Service().Create(ctx, service)
}

// GenerateSSHKeyPair generates an RSA SSH key pair
func GenerateSSHKeyPair() (privateKeyStr string, publicKeyStr string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	publicKeyStr = string(ssh.MarshalAuthorizedKey(publicKey))

	return string(privateKeyPEM), publicKeyStr, nil
}

// GetProvisionScript returns the provision script for a server
// This is used for custom servers that need to run the provision script manually
// If teamID is provided, it validates team ownership
func (s *Service) GetProvisionScript(ctx context.Context, serverID string, teamID ...string) (string, error) {
	var server *models.Server
	var err error

	// Find server with optional team validation
	if len(teamID) > 0 && teamID[0] != "" {
		server, err = s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID[0])
	} else {
		server, err = s.repos.Server().FindByID(ctx, serverID)
	}
	if err != nil {
		return "", err
	}

	if server == nil {
		return "", errors.New("server not found")
	}

	// Only custom servers or archived servers can get provision script
	if server.Provider != types.ProviderCustom && server.ArchivedAt == nil {
		return "", nil
	}

	// Generate the provision script that authorizes our management key
	script := s.generateAuthorizeKeyScript(server)
	return script, nil
}

// shellEscape escapes a string for safe inclusion in a single-quoted shell string.
// It replaces any single quote with the sequence '\” (end quote, escaped quote, start quote).
func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", "'\\''")
}

// generateAuthorizeKeyScript generates a bash script to authorize the management key.
//
// Two correctness invariants, both learned the hard way from a real
// customer hitting "ssh: unable to authenticate" after running the
// quick command:
//
//  1. The script MUST run as root. Launch's backend SSH's into the box
//     as the root user (server.RootUsername() returns "root" for the
//     custom_server provider) and presents server.PrivateKey. The
//     matching public key therefore has to live in
//     /root/.ssh/authorized_keys. If the user SSH's in as ubuntu/admin
//     and pipes to bash without sudo, ~/.ssh expands to /home/<user>/
//     and the key lands in the wrong place — the script reports
//     "Management key authorized successfully" but the backend's later
//     Try Connection fails with no path to recover from the user's
//     terminal output. So: refuse to run as non-root, loudly.
//
//  2. Write to /root/.ssh/authorized_keys explicitly, never ~/.ssh.
//     Even when EUID is 0, $HOME can still be set to something else
//     (e.g. when a user did `sudo -E bash`, or when piped through some
//     CI shells). Hard-coding the path removes the variable entirely.
func (s *Service) generateAuthorizeKeyScript(server *models.Server) string {
	// TrimSpace is load-bearing. ssh.MarshalAuthorizedKey appends a
	// trailing newline, which would otherwise be baked into the script
	// as `PUBLIC_KEY="ssh-rsa AAAA...==\n"`. When bash later expands
	// "$PUBLIC_KEY" for `grep -qF`, grep treats a multi-line pattern
	// as OR-ed lines — and one of those lines is empty. The empty
	// pattern matches every line in any non-empty file, so grep -qF
	// silently always succeeds, the script reports "Management key
	// already authorized", and the `echo >> authorized_keys` write
	// never runs. That bug stranded a customer's server in
	// awaiting_connection — every retry no-op'd. Trim the key so the
	// shell variable is a clean single-line value.
	publicKey := strings.TrimSpace(string(server.PublicKey))
	escapedName := shellEscape(server.Name)

	script := `#!/bin/bash
set -e

# Launch Server Provisioning Script
# Server: '%s'

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root."
    echo
    echo "Re-run the provision command from your terminal with sudo, e.g.:"
    echo "  wget --no-verbose -O - 'https://...' | sudo bash"
    echo "or, if you're already in a shell, prefix it with sudo:"
    echo "  sudo bash provision.sh"
    exit 1
fi

echo "Authorizing Launch management key..."

# Install into /root/.ssh explicitly — Launch connects as root, so the
# key must land in root's authorized_keys regardless of $HOME at run
# time. Using ~/.ssh would silently install under the wrong user when
# the script is run with "sudo -E bash" or similar HOME-preserving
# invocations.
mkdir -p /root/.ssh
chmod 700 /root/.ssh
chown root:root /root/.ssh

PUBLIC_KEY="%s"

# grep -F: treat $PUBLIC_KEY as a fixed string (it contains +, /, =
# from base64 that grep would otherwise read as regex metacharacters).
# --: end option processing in case the key happens to start with -.
if ! grep -qF -- "$PUBLIC_KEY" /root/.ssh/authorized_keys 2>/dev/null; then
    echo "$PUBLIC_KEY" >> /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    chown root:root /root/.ssh/authorized_keys
    echo "Management key authorized successfully."
else
    echo "Management key already authorized."
fi

echo "Provisioning script completed."
`
	return fmt.Sprintf(script, escapedName, publicKey)
}

// RetryProvision re-runs a failed server's pipeline from wherever it died.
// Two distinct failure points to handle:
//
//   - Create-on-provider failed: there's no droplet/instance on the cloud
//     side, and the server never SSH-connected. Re-dispatch CreateOnProvider
//     so a fresh upstream resource gets created.
//   - Provisioning failed: the droplet exists, SSH succeeded, but one of the
//     provision steps blew up. Re-dispatch ProvisionServer.
//
// Picking the right branch is what makes the UI "Try again" button do the
// expected thing — previously we always tried to provision and bailed with
// "server has not connected successfully" on the more common case.
//
// Signature matches ActionFunc.
func (s *Service) RetryProvision(ctx context.Context, serverID, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	if server.Status != types.ServerStatusFailed {
		return errors.New("server is not in failed state")
	}

	// Clear the previous failure reason so the UI no longer shows it once the
	// new attempt starts. Best-effort — if it fails we still proceed.
	if err := s.repos.Server().UpdateFields(ctx, serverID, map[string]any{
		"status":          types.ServerStatusStarting,
		"provision_error": nil,
		"progress":        0,
	}); err != nil {
		s.LogError(err, "Failed to reset server state before retry", "server_id", serverID)
	}

	// Gather SSH key IDs once — both job types accept them.
	sshKeys, err := s.repos.SSHKey().FindByServer(ctx, serverID)
	if err != nil {
		s.LogError(err, "Failed to get SSH keys for retry", "server_id", serverID)
	}
	var sshKeyIDs []string
	for _, key := range sshKeys {
		sshKeyIDs = append(sshKeyIDs, key.ID)
	}

	// Pick the right branch.
	// A cloud-provider-backed server that hasn't connected needs the cloud
	// resource (re)created first. Custom servers don't have a provider id, so
	// they fall through to the provision-only retry.
	needsCloudCreate := !server.Connected && server.ServerProviderID != nil && *server.ServerProviderID != ""

	if needsCloudCreate {
		task, err := jobs.NewCreateOnProviderTask(serverID, teamID, *server.ServerProviderID, nil, sshKeyIDs)
		if err != nil {
			return fmt.Errorf("failed to create cloud-provider task: %w", err)
		}
		if err := s.EnqueueTask(task); err != nil {
			return fmt.Errorf("failed to enqueue cloud-provider task: %w", err)
		}
		activity.RecordEvent(ctx, "provision_retry", "", server, "Cloud provider create was retried")
		return nil
	}

	task, err := jobs.NewProvisionServerTask(serverID, teamID, nil, sshKeyIDs)
	if err != nil {
		return fmt.Errorf("failed to create provision task: %w", err)
	}
	if err := s.EnqueueTask(task); err != nil {
		return fmt.Errorf("failed to enqueue provision task: %w", err)
	}
	activity.RecordEvent(ctx, "provision_retry", "", server, "Server provisioning was retried")
	return nil
}

// BackfillDetectedOS enqueues a job that SSHes into the server, parses
// /etc/os-release + uname, and writes the detected_os_* / detected_arch
// / detected_kernel columns. Used for:
//
//   - Legacy servers provisioned before the detect_os step existed
//     (their detected_* fields are all NULL).
//   - Refreshing the cached facts after a distro/kernel upgrade.
//
// Synchronous part is just enqueue-and-return; the actual SSH and DB
// update happen in the worker so the HTTP request isn't gated on a
// potentially-slow network round-trip. Result broadcasts as
// server.updated when the job finishes.
//
// Signature matches ActionFunc.
func (s *Service) BackfillDetectedOS(ctx context.Context, serverID, teamID, userID string) error {
	_ = userID
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}
	if !server.Connected {
		return fiberutil.Validation("Server is not connected; cannot detect OS facts yet.")
	}
	return s.MustDispatch(func() (*asynq.Task, error) {
		return jobs.NewBackfillDetectedOSTask(serverID)
	})
}

// RunVulnerabilityAudit runs a security vulnerability audit on a server
func (s *Service) RunVulnerabilityAudit(ctx context.Context, serverID, teamID, userID string, emailRecipient *string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	if !server.IsConnected() {
		return ErrServerNotConnected
	}

	task, err := jobs.NewVulnerabilityAuditTask(server.ID, teamID, &userID, emailRecipient)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "vulnerability_audit_started", userID, server, "Vulnerability audit was initiated")

	return s.EnqueueTask(task)
}

// guardDockerProjectsExist returns a 422 validation error when the server
// still has live docker projects. Hard-blocks DeleteServer so the user
// must tear down projects (and their workloads) first.
//
// Implementation note: we query the `docker_projects` table directly via
// the service's *gorm.DB handle rather than going through the docker
// module's ProjectRepository. The docker module imports server (for
// server models / types), so a reverse import would create a cycle.
// Counting one column with a soft-delete-aware WHERE keeps this cheap
// — the query is `SELECT 1 FROM docker_projects WHERE server_id = ?
// AND deleted_at IS NULL LIMIT 1` style, scanning the index docker
// migrations already create on server_id.
//
// The companion frontend disables the Delete button when
// `projects_count > 0` is exposed on the server response, so this
// server-side check is the backstop — never the primary UX.
func (s *Service) guardDockerProjectsExist(ctx context.Context, serverID string) error {
	if !s.HasDB() {
		return nil
	}
	var count int64
	if err := s.DB().WithContext(ctx).
		Table("docker_projects").
		Where("server_id = ? AND deleted_at IS NULL", serverID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count docker projects: %w", err)
	}
	if count == 0 {
		return nil
	}
	noun := "project"
	if count > 1 {
		noun = "projects"
	}
	return fiberutil.Validation(fmt.Sprintf(
		"This server still has %d Docker %s. Remove every project (and the apps / compose stacks / databases inside it) before deleting the server.",
		count, noun,
	))
}
