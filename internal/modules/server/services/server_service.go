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

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// ListServers returns all servers for a team
func (s *Service) ListServers(ctx context.Context, teamID string) ([]models.Server, error) {
	return s.repos.Server().FindAllByTeam(ctx, teamID)
}

// ListArchivedServers returns all archived servers for a team
func (s *Service) ListArchivedServers(ctx context.Context, teamID string) ([]models.Server, error) {
	return s.repos.Server().FindArchivedByTeam(ctx, teamID)
}

// ListServersPaginated returns servers with pagination
func (s *Service) ListServersPaginated(ctx context.Context, teamID string, page, perPage int) (*repository.PaginatedResult[models.Server], error) {
	return s.repos.Server().FindAllByTeamPaginated(ctx, teamID, page, perPage)
}

// GetServer returns a server by ID
func (s *Service) GetServer(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
}

// GetServerWithRelations returns a server with all relations
func (s *Service) GetServerWithRelations(ctx context.Context, id, teamID string) (*models.Server, error) {
	return s.repos.Server().FindWithRelations(ctx, id, teamID)
}

// CreateServer creates a new server
func (s *Service) CreateServer(ctx context.Context, teamID, userID string, req *dto.CreateServerRequest) (*models.Server, error) {
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

	server := &models.Server{
		Name:             req.Name,
		Description:      req.Description,
		Provider:         provider,
		Type:             &serverTypeStr,
		OperatingSystem:  &osStr,
		Status:           types.ServerStatusNew,
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

	// Dispatch appropriate job based on provider type
	if provider == types.ProviderCustom {
		// Custom servers skip cloud creation, go straight to waiting for connection
		if err := s.dispatchWaitForConnectionJob(server, req.SSHKeyIDs); err != nil {
			s.LogError(err, "Failed to dispatch wait for connection job", "server_id", server.ID)
		}
	} else {
		// Cloud servers need to be created on the provider first
		if err := s.dispatchCreateOnProviderJob(server, req.CredentialID, req.SSHKeyIDs); err != nil {
			s.LogError(err, "Failed to dispatch create on provider job", "server_id", server.ID)
		}
	}

	return server, nil
}

// UpdateServer updates a server
func (s *Service) UpdateServer(ctx context.Context, id, teamID string, req *dto.UpdateServerRequest) (*models.Server, error) {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	activity.RecordEvent(ctx, "updated", "", server, "Server was updated")

	s.broadcastServerUpdate(server)

	return server, nil
}

// DeleteServer deletes a server
func (s *Service) DeleteServer(ctx context.Context, id, teamID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	activity.RecordEvent(ctx, "deleted", "", server, "Server deletion requested")

	if err := s.repos.Server().UpdateStatus(ctx, id, types.ServerStatusDeleting); err != nil {
		return err
	}

	if server.Provider != types.ProviderCustom {
		if err := s.dispatchDeleteJob(server); err != nil {
			s.LogError(err, "Failed to dispatch delete job", "server_id", server.ID)
		}
	}

	if server.Provider == types.ProviderCustom {
		return s.repos.Server().Delete(ctx, id)
	}

	return nil
}

// ArchiveServer archives a server
func (s *Service) ArchiveServer(ctx context.Context, id, teamID string) error {
	_, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.repos.Server().Archive(ctx, id)
}

// UnarchiveServer unarchives a server
func (s *Service) UnarchiveServer(ctx context.Context, id, teamID string) error {
	_, err := s.repos.Server().FindByIDAndTeam(ctx, id, teamID)
	if err != nil {
		return err
	}

	return s.repos.Server().Unarchive(ctx, id)
}

// RebootServer initiates a server reboot
func (s *Service) RebootServer(ctx context.Context, id, teamID string) error {
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

// ConnectServer tests the connection to a server
func (s *Service) ConnectServer(ctx context.Context, id, teamID string) error {
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
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewCreateOnProviderTask(server.ID, server.TeamID, serverProviderID, nil, sshKeyIDs)
	if err != nil {
		return err
	}

	return s.EnqueueTaskWithOptions(task)
}

func (s *Service) dispatchWaitForConnectionJob(server *models.Server, sshKeyIDs []string) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewWaitForServerToConnectTask(server.ID, server.TeamID, "", nil, sshKeyIDs)
	if err != nil {
		return err
	}

	return s.EnqueueTaskWithOptions(task)
}

func (s *Service) dispatchDeleteJob(server *models.Server) error {
	if !s.HasQueue() {
		return ErrQueueNotConfigured
	}

	task, err := jobs.NewDeleteServerTask(server.ID, server.TeamID, nil)
	if err != nil {
		return err
	}

	return s.EnqueueTask(task)
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

// generateAuthorizeKeyScript generates a bash script to authorize the management key
func (s *Service) generateAuthorizeKeyScript(server *models.Server) string {
	publicKey := string(server.PublicKey)
	escapedName := shellEscape(server.Name)

	script := `#!/bin/bash
set -e

# Launch Server Provisioning Script
# Server: '%s'

echo "Authorizing Launch management key..."

# Create .ssh directory if it doesn't exist
mkdir -p ~/.ssh
chmod 700 ~/.ssh

# Add the public key to authorized_keys if not already present
PUBLIC_KEY="%s"

if ! grep -q "$PUBLIC_KEY" ~/.ssh/authorized_keys 2>/dev/null; then
    echo "$PUBLIC_KEY" >> ~/.ssh/authorized_keys
    chmod 600 ~/.ssh/authorized_keys
    echo "Management key authorized successfully."
else
    echo "Management key already authorized."
fi

# Notify the server that provisioning is complete
echo "Provisioning script completed."
`
	return fmt.Sprintf(script, escapedName, publicKey)
}

// RetryProvision retries the provisioning of a failed server
// This only works if the server has connected successfully but provisioning failed
func (s *Service) RetryProvision(ctx context.Context, serverID, teamID string) error {
	server, err := s.repos.Server().FindByIDAndTeam(ctx, serverID, teamID)
	if err != nil {
		return err
	}

	// Only allow retry if server status is failed
	if server.Status != types.ServerStatusFailed {
		return errors.New("server is not in failed state")
	}

	// Only allow retry if server has connected (SSH connection was successful)
	if !server.Connected {
		return errors.New("server has not connected successfully")
	}

	// Get SSH keys attached to this server
	sshKeys, err := s.repos.SSHKey().FindByServer(ctx, serverID)
	if err != nil {
		s.LogError(err, "Failed to get SSH keys for retry", "server_id", serverID)
	}

	var sshKeyIDs []string
	for _, key := range sshKeys {
		sshKeyIDs = append(sshKeyIDs, key.ID)
	}

	// Dispatch the provision job again
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
